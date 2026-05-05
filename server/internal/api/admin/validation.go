package admin

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	ifaceRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,14}$`)
	labelRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
)

func cleanText(label, value string, max int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if len(value) > max {
		return "", fmt.Errorf("%s is too long", label)
	}
	if hasControl(value) {
		return "", fmt.Errorf("%s contains invalid control characters", label)
	}
	return value, nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func validateInterfaceName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !ifaceRe.MatchString(value) {
		return "", fmt.Errorf("interface_name must be 1-15 chars and contain only letters, numbers, dot, underscore, or dash")
	}
	return value, nil
}

func validatePort(label string, port int) (int, error) {
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s must be between 1 and 65535", label)
	}
	return port, nil
}

func validateHost(label, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	if len(value) > 253 || hasControl(value) || strings.ContainsAny(value, `/\,"' `) {
		return "", fmt.Errorf("%s is not a valid host name or IP address", label)
	}
	host := strings.Trim(value, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	labels := strings.Split(host, ".")
	for _, part := range labels {
		if !labelRe.MatchString(part) {
			return "", fmt.Errorf("%s is not a valid host name or IP address", label)
		}
	}
	return strings.ToLower(host), nil
}

func validateIPv4Addr(label, value string) (string, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("%s must be an IPv4 address", label)
	}
	return addr.String(), nil
}

func validateIPv4Prefix(label, value string, minBits, maxBits int) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil || !prefix.Addr().Is4() {
		return "", fmt.Errorf("%s must be an IPv4 CIDR prefix", label)
	}
	bits := prefix.Bits()
	if bits < minBits || bits > maxBits {
		return "", fmt.Errorf("%s prefix must be between /%d and /%d", label, minBits, maxBits)
	}
	return prefix.Masked().String(), nil
}

func validateResourceAddress(kind string, ip string, mask *int) (string, *int, error) {
	switch kind {
	case "host":
		addr, err := validateIPv4Addr("ip_address", ip)
		return addr, nil, err
	case "network":
		if mask == nil {
			return "", nil, fmt.Errorf("mask is required for network resources")
		}
		cidr, err := validateIPv4Prefix("resource network", fmt.Sprintf("%s/%d", strings.TrimSpace(ip), *mask), 1, 32)
		if err != nil {
			return "", nil, err
		}
		prefix, _ := netip.ParsePrefix(cidr)
		bits := prefix.Bits()
		return prefix.Addr().String(), &bits, nil
	default:
		return "", nil, fmt.Errorf("type must be host or network")
	}
}

func validateDNSList(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if ip := net.ParseIP(strings.Trim(part, "[]")); ip != nil {
			out = append(out, ip.String())
			continue
		}
		host, err := validateHost("dns", part)
		if err != nil {
			return "", err
		}
		out = append(out, host)
	}
	return strings.Join(out, ","), nil
}

func validateWireGuardPublicKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("public_key must be a base64 WireGuard public key")
	}
	return value, nil
}

func validateSettingValue(key, value string) (string, error) {
	switch key {
	case "vpn_name":
		return cleanText("vpn_name", value, 80)
	case "session_timeout":
		return validateIntSetting(key, value, 10, 86400)
	case "keepalive_interval":
		return validateIntSetting(key, value, 5, 3600)
	case "webauthn_rp_id":
		return validateHost(key, value)
	case "webauthn_rp_name":
		return cleanText(key, value, 80)
	case "webauthn_origin":
		return validateOrigin(value)
	case "push_auth_enabled":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes", "on":
			return "true", nil
		case "false", "0", "no", "off":
			return "false", nil
		default:
			return "", fmt.Errorf("push_auth_enabled must be true or false")
		}
	case "push_auth_api_key":
		if strings.TrimSpace(value) == "" || value == configuredSecretMarker {
			return "", nil
		}
		return cleanText(key, value, 512)
	default:
		return "", fmt.Errorf("unknown setting %q", key)
	}
}

func validateIntSetting(key, value string, min, max int) (string, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < min || n > max {
		return "", fmt.Errorf("%s must be a number between %d and %d", key, min, max)
	}
	return strconv.Itoa(n), nil
}

func validateOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("webauthn_origin must be an origin URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("webauthn_origin must use http or https")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("webauthn_origin must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("webauthn_origin must not include query or fragment")
	}
	return u.Scheme + "://" + u.Host, nil
}
