package wireguard

import (
	"encoding/base64"
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerEntry is a minimal peer descriptor used for full config sync.
type PeerEntry struct {
	PublicKey  string
	AssignedIP string // without prefix, e.g. "10.8.0.2"
}

// Manager wraps wgctrl to add/remove peers on the server WireGuard interface.
type Manager struct {
	iface  string
	client *wgctrl.Client
}

func NewManager(iface string) (*Manager, error) {
	c, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("wgctrl: %w", err)
	}
	return &Manager{iface: iface, client: c}, nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}

// ServerPublicKey returns the server's WireGuard public key in base64.
func (m *Manager) ServerPublicKey() (string, error) {
	dev, err := m.client.Device(m.iface)
	if err != nil {
		return "", fmt.Errorf("get device %s: %w", m.iface, err)
	}
	return dev.PublicKey.String(), nil
}

// GenerateKeypair creates a fresh WireGuard private/public key pair.
// Returns (privateKeyBase64, publicKeyBase64).
func GenerateKeypair() (string, string, error) {
	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	return priv.String(), priv.PublicKey().String(), nil
}

// AddPeer adds a client peer to the WireGuard interface with the given
// public key and /32 allowed IP.
func (m *Manager) AddPeer(clientPubKeyB64, assignedIP string) error {
	pub, err := parseKey(clientPubKeyB64)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	_, ipNet, err := net.ParseCIDR(assignedIP + "/32")
	if err != nil {
		return fmt.Errorf("parse ip %q: %w", assignedIP, err)
	}

	ka := 25 * time.Second
	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   pub,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  []net.IPNet{*ipNet},
				PersistentKeepaliveInterval: &ka,
			},
		},
	}
	if err := m.client.ConfigureDevice(m.iface, cfg); err != nil {
		return fmt.Errorf("configure peer: %w", err)
	}
	return nil
}

// RemovePeer removes a peer by public key from the WireGuard interface.
func (m *Manager) RemovePeer(clientPubKeyB64 string) error {
	pub, err := parseKey(clientPubKeyB64)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	cfg := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: pub,
				Remove:    true,
			},
		},
	}
	if err := m.client.ConfigureDevice(m.iface, cfg); err != nil {
		return fmt.Errorf("remove peer: %w", err)
	}
	return nil
}

// SyncAllPeers atomically replaces the full peer list on the interface.
// Call this after any add/remove to ensure the interface config is consistent
// with the database state.
func (m *Manager) SyncAllPeers(peers []PeerEntry) error {
	ka := 25 * time.Second
	pcs := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, p := range peers {
		pub, err := parseKey(p.PublicKey)
		if err != nil {
			continue
		}
		_, ipNet, err := net.ParseCIDR(p.AssignedIP + "/32")
		if err != nil {
			continue
		}
		pcs = append(pcs, wgtypes.PeerConfig{
			PublicKey:                   pub,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  []net.IPNet{*ipNet},
			PersistentKeepaliveInterval: &ka,
		})
	}
	cfg := wgtypes.Config{
		ReplacePeers: true,
		Peers:        pcs,
	}
	if err := m.client.ConfigureDevice(m.iface, cfg); err != nil {
		return fmt.Errorf("sync peers on %s: %w", m.iface, err)
	}
	return nil
}

// PeerHandshakeAge returns how long ago the peer last completed a handshake.
// Returns (0, nil) if the peer has never completed a handshake.
// Returns an error if the peer is not found on the interface.
func (m *Manager) PeerHandshakeAge(pubKeyB64 string) (time.Duration, error) {
	pub, err := parseKey(pubKeyB64)
	if err != nil {
		return 0, err
	}
	dev, err := m.client.Device(m.iface)
	if err != nil {
		return 0, fmt.Errorf("get device: %w", err)
	}
	for _, p := range dev.Peers {
		if p.PublicKey == pub {
			if p.LastHandshakeTime.IsZero() {
				return 0, nil
			}
			return time.Since(p.LastHandshakeTime), nil
		}
	}
	return 0, fmt.Errorf("peer not found on %s", m.iface)
}

// ConfigureInterface sets the private key and listen port on the interface using wgctrl.
// Call this after SetupInterface.
func (m *Manager) ConfigureInterface(privateKeyB64 string, listenPort int) error {
	priv, err := parseKey(privateKeyB64)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	cfg := wgtypes.Config{
		PrivateKey: &priv,
		ListenPort: &listenPort,
	}
	if err := m.client.ConfigureDevice(m.iface, cfg); err != nil {
		return fmt.Errorf("configure interface: %w", err)
	}
	return nil
}

func parseKey(b64 string) (wgtypes.Key, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("base64 decode: %w", err)
	}
	return wgtypes.NewKey(raw)
}
