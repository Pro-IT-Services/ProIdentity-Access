package config

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// TunnelConfig represents a complete WireGuard tunnel configuration.
type TunnelConfig struct {
	ID        string
	OwnerID   string
	Name      string
	Interface InterfaceConfig
	Peers     []PeerConfig
}

// InterfaceConfig holds the [Interface] section of a WireGuard config.
type InterfaceConfig struct {
	PrivateKey string
	Addresses  []string // CIDR notation, e.g. "10.0.0.2/24"
	DNS        []string
	MTU        int
	ListenPort int
}

// PeerConfig holds a [Peer] section of a WireGuard config.
type PeerConfig struct {
	PublicKey           string
	PresharedKey        string
	AllowedIPs          []string
	Endpoint            string
	PersistentKeepalive int
}

// Format serializes the TunnelConfig back to WireGuard .conf format.
// This is the inverse of ParseString and is used for config export/upload.
func (c *TunnelConfig) Format() string {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	if c.Interface.PrivateKey != "" {
		fmt.Fprintf(&sb, "PrivateKey = %s\n", c.Interface.PrivateKey)
	}
	if len(c.Interface.Addresses) > 0 {
		fmt.Fprintf(&sb, "Address = %s\n", strings.Join(c.Interface.Addresses, ", "))
	}
	if len(c.Interface.DNS) > 0 {
		fmt.Fprintf(&sb, "DNS = %s\n", strings.Join(c.Interface.DNS, ", "))
	}
	if c.Interface.MTU > 0 && c.Interface.MTU != 1420 {
		fmt.Fprintf(&sb, "MTU = %d\n", c.Interface.MTU)
	}
	if c.Interface.ListenPort > 0 {
		fmt.Fprintf(&sb, "ListenPort = %d\n", c.Interface.ListenPort)
	}

	for _, peer := range c.Peers {
		sb.WriteString("\n[Peer]\n")
		if peer.PublicKey != "" {
			fmt.Fprintf(&sb, "PublicKey = %s\n", peer.PublicKey)
		}
		if peer.PresharedKey != "" {
			fmt.Fprintf(&sb, "PresharedKey = %s\n", peer.PresharedKey)
		}
		if peer.Endpoint != "" {
			fmt.Fprintf(&sb, "Endpoint = %s\n", peer.Endpoint)
		}
		if len(peer.AllowedIPs) > 0 {
			fmt.Fprintf(&sb, "AllowedIPs = %s\n", strings.Join(peer.AllowedIPs, ", "))
		}
		if peer.PersistentKeepalive > 0 {
			fmt.Fprintf(&sb, "PersistentKeepalive = %d\n", peer.PersistentKeepalive)
		}
	}

	return sb.String()
}

// GenerateUAPI returns the UAPI text protocol configuration for wireguard-go.
func (c *TunnelConfig) GenerateUAPI() string {
	var sb strings.Builder

	if privKeyHex, err := keyBase64ToHex(c.Interface.PrivateKey); err == nil {
		fmt.Fprintf(&sb, "private_key=%s\n", privKeyHex)
	}

	if c.Interface.ListenPort > 0 {
		fmt.Fprintf(&sb, "listen_port=%d\n", c.Interface.ListenPort)
	}

	if len(c.Peers) > 0 {
		fmt.Fprintf(&sb, "replace_peers=true\n")
		for _, peer := range c.Peers {
			pubKeyHex, err := keyBase64ToHex(peer.PublicKey)
			if err != nil {
				continue
			}
			fmt.Fprintf(&sb, "public_key=%s\n", pubKeyHex)

			if peer.PresharedKey != "" {
				if pskHex, err := keyBase64ToHex(peer.PresharedKey); err == nil {
					fmt.Fprintf(&sb, "preshared_key=%s\n", pskHex)
				}
			}

			if peer.Endpoint != "" {
				fmt.Fprintf(&sb, "endpoint=%s\n", peer.Endpoint)
			}

			if len(peer.AllowedIPs) > 0 {
				fmt.Fprintf(&sb, "replace_allowed_ips=true\n")
				for _, aip := range peer.AllowedIPs {
					fmt.Fprintf(&sb, "allowed_ip=%s\n", strings.TrimSpace(aip))
				}
			}

			if peer.PersistentKeepalive > 0 {
				fmt.Fprintf(&sb, "persistent_keepalive_interval=%d\n", peer.PersistentKeepalive)
			}
		}
	}

	return sb.String()
}

// AllPeerAllowedIPs returns all AllowedIPs from all peers.
func (c *TunnelConfig) AllPeerAllowedIPs() []string {
	var ips []string
	for _, peer := range c.Peers {
		ips = append(ips, peer.AllowedIPs...)
	}
	return ips
}

// MaskedPrivateKey returns a redacted version of the private key safe to send to the GUI.
func (c *TunnelConfig) MaskedPrivateKey() string {
	if len(c.Interface.PrivateKey) < 8 {
		return "***"
	}
	return c.Interface.PrivateKey[:4] + "****" + c.Interface.PrivateKey[len(c.Interface.PrivateKey)-4:]
}

func keyBase64ToHex(b64Key string) (string, error) {
	b64Key = strings.TrimSpace(b64Key)
	data, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(b64Key)
		if err != nil {
			return "", fmt.Errorf("invalid base64 key: %w", err)
		}
	}
	if len(data) != 32 {
		return "", fmt.Errorf("expected 32-byte key, got %d bytes", len(data))
	}
	return hex.EncodeToString(data), nil
}
