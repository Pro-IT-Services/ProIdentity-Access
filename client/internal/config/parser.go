package config

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse reads a WireGuard .conf file and returns a TunnelConfig.
// It supports multiple [Peer] sections.
func Parse(r io.Reader) (*TunnelConfig, error) {
	cfg := &TunnelConfig{}
	var currentSection string
	var currentPeer *PeerConfig

	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(line[1 : len(line)-1])
			switch section {
			case "interface":
				currentSection = "interface"
				currentPeer = nil
			case "peer":
				// Save previous peer if any
				if currentPeer != nil {
					cfg.Peers = append(cfg.Peers, *currentPeer)
				}
				currentPeer = &PeerConfig{}
				currentSection = "peer"
			default:
				currentSection = "unknown"
				currentPeer = nil
			}
			continue
		}

		// Key = Value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: invalid syntax %q", lineNum, line)
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip inline comments
		if idx := strings.Index(val, " #"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}

		switch currentSection {
		case "interface":
			parseInterfaceKey(&cfg.Interface, key, val) // unknown keys silently ignored
		case "peer":
			if currentPeer == nil {
				currentPeer = &PeerConfig{}
			}
			parsePeerKey(currentPeer, key, val) // unknown keys silently ignored
		default:
			// key before any section — ignore (e.g. Name = ...)
		}
	}

	// Append last peer
	if currentPeer != nil {
		cfg.Peers = append(cfg.Peers, *currentPeer)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	if cfg.Interface.PrivateKey == "" {
		return nil, fmt.Errorf("[Interface] PrivateKey is required")
	}
	if len(cfg.Interface.Addresses) == 0 {
		return nil, fmt.Errorf("[Interface] Address is required")
	}

	// Default MTU
	if cfg.Interface.MTU == 0 {
		cfg.Interface.MTU = 1420
	}

	return cfg, nil
}

// ParseString parses a WireGuard config from a string.
func ParseString(s string) (*TunnelConfig, error) {
	return Parse(strings.NewReader(s))
}

func parseInterfaceKey(iface *InterfaceConfig, key, val string) error {
	switch strings.ToLower(key) {
	case "privatekey":
		iface.PrivateKey = val
	case "address":
		for _, addr := range strings.Split(val, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				iface.Addresses = append(iface.Addresses, addr)
			}
		}
	case "dns":
		for _, d := range strings.Split(val, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				iface.DNS = append(iface.DNS, d)
			}
		}
	case "mtu":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid MTU %q: %w", val, err)
		}
		iface.MTU = n
	case "listenport":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid ListenPort %q: %w", val, err)
		}
		iface.ListenPort = n
	case "postup", "postdown", "preup", "predown", "table", "saveconfig":
		// Accepted but ignored — these are script hooks not applicable to userspace
	default:
		return fmt.Errorf("unknown Interface key %q", key)
	}
	return nil
}

func parsePeerKey(peer *PeerConfig, key, val string) error {
	switch strings.ToLower(key) {
	case "publickey":
		peer.PublicKey = val
	case "presharedkey":
		peer.PresharedKey = val
	case "allowedips":
		for _, ip := range strings.Split(val, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				peer.AllowedIPs = append(peer.AllowedIPs, ip)
			}
		}
	case "endpoint":
		peer.Endpoint = val
	case "persistentkeepalive":
		if strings.EqualFold(val, "off") || val == "0" {
			peer.PersistentKeepalive = 0
			break
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid PersistentKeepalive %q: %w", val, err)
		}
		peer.PersistentKeepalive = n
	default:
		return fmt.Errorf("unknown Peer key %q", key)
	}
	return nil
}
