package requestip

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// ClientIP returns the best client IP for logs, rate limits, and auth context.
// Proxy headers are trusted only when the direct peer is explicitly trusted.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	peer := PeerHost(r.RemoteAddr)
	if TrustedForwarder(peer) {
		if ip := forwardedHeaderIP(r.Header.Values("Forwarded")); ip != "" {
			return ip
		}
		if os.Getenv("PROIDENTITY_DISABLE_X_FORWARDED_FOR") != "1" {
			if ip := forwardedForIP(r.Header.Values("X-Forwarded-For")); ip != "" {
				return ip
			}
		}
		if ip := singleIPHeader(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	if peer == "" {
		return r.RemoteAddr
	}
	return peer
}

func PeerHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return strings.Trim(addr, "[]")
	}
	return strings.Trim(host, "[]")
}

func singleIPHeader(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if net.ParseIP(value) == nil {
		return ""
	}
	return value
}

func forwardedForIP(values []string) string {
	parts := splitHeaderList(values)
	return rightmostUntrusted(parts)
}

func forwardedHeaderIP(values []string) string {
	var ips []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			for _, item := range strings.Split(part, ";") {
				k, v, ok := strings.Cut(strings.TrimSpace(item), "=")
				if !ok || !strings.EqualFold(strings.TrimSpace(k), "for") {
					continue
				}
				v = strings.Trim(strings.TrimSpace(v), `"`)
				if strings.HasPrefix(v, "[") {
					if end := strings.Index(v, "]"); end >= 0 {
						v = v[1:end]
					}
				} else if host, _, err := net.SplitHostPort(v); err == nil {
					v = host
				}
				ips = append(ips, v)
			}
		}
	}
	return rightmostUntrusted(ips)
}

func splitHeaderList(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func rightmostUntrusted(values []string) string {
	var fallback string
	for i := len(values) - 1; i >= 0; i-- {
		ipText := strings.Trim(strings.TrimSpace(values[i]), "[]")
		if host, _, err := net.SplitHostPort(ipText); err == nil {
			ipText = host
		}
		ip := net.ParseIP(ipText)
		if ip == nil {
			continue
		}
		clean := ip.String()
		if fallback == "" {
			fallback = clean
		}
		if !TrustedForwarder(clean) {
			return clean
		}
	}
	return fallback
}

func TrustedForwarder(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	if ip.IsLoopback() && os.Getenv("PROIDENTITY_TRUST_LOOPBACK_PROXY") == "1" {
		return true
	}
	for _, raw := range strings.Split(os.Getenv("PROIDENTITY_TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if trustedIP := net.ParseIP(strings.Trim(raw, "[]")); trustedIP != nil {
			if trustedIP.Equal(ip) {
				return true
			}
			continue
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err == nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}
