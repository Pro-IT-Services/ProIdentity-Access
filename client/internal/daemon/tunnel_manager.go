package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wg-client/internal/config"
	"wg-client/internal/ipc"

	"github.com/google/uuid"
)

// TunnelManager manages the lifecycle of all WireGuard tunnels.
// It is the core of the daemon and implements ipc.Handler.
type TunnelManager struct {
	storageDir string
	broadcast  func(ipc.Event)

	mu      sync.RWMutex
	tunnels map[string]*Tunnel // keyed by ID

	keyMu   sync.RWMutex
	encKeys map[string][]byte // 32-byte AES-256 key by OS owner; nil until set by GUI
}

// NewTunnelManager creates a TunnelManager that persists configs to storageDir
// and uses broadcast to push events to connected GUI clients.
func NewTunnelManager(storageDir string, broadcast func(ipc.Event)) (*TunnelManager, error) {
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	if err := hardenStorageACL(storageDir); err != nil {
		log.Printf("warn: harden storage ACL %s: %v", storageDir, err)
	}
	m := &TunnelManager{
		storageDir: storageDir,
		broadcast:  broadcast,
		tunnels:    make(map[string]*Tunnel),
		encKeys:    make(map[string][]byte),
	}
	return m, nil
}

// --- ipc.Handler implementation ---

func (m *TunnelManager) ListTunnels(principal ipc.Principal) ([]ipc.TunnelInfo, error) {
	ownerID := ownerForPrincipal(principal)
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []ipc.TunnelInfo
	for _, t := range m.tunnels {
		if !m.tunnelVisibleToOwner(t, ownerID) {
			continue
		}
		list = append(list, t.Info())
	}
	return list, nil
}

func (m *TunnelManager) ImportTunnel(principal ipc.Principal, name, configContent string) (*ipc.TunnelInfo, error) {
	ownerID := ownerForPrincipal(principal)
	if !m.encryptionKeySet(ownerID) {
		return nil, fmt.Errorf("persistent tunnel storage encryption key is not set")
	}

	cfg, err := config.ParseString(configContent)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ID = uuid.New().String()
	cfg.OwnerID = ownerID
	if name != "" {
		cfg.Name = name
	} else if cfg.Name == "" {
		cfg.Name = "tunnel-" + cfg.ID[:8]
	}

	t := NewTunnel(cfg)

	m.mu.Lock()
	m.tunnels[cfg.ID] = t
	m.mu.Unlock()

	if err := m.persistConfig(cfg); err != nil {
		log.Printf("warn: persist config %s: %v", cfg.ID, err)
	}

	info := t.Info()
	m.emitChanged(info)
	return &info, nil
}

// ImportEphemeralTunnel imports a tunnel without writing it to disk.
// Used for managed VPN sessions that must not outlive the session.
func (m *TunnelManager) ImportEphemeralTunnel(principal ipc.Principal, name, configContent string) (*ipc.TunnelInfo, error) {
	ownerID := ownerForPrincipal(principal)
	cfg, err := config.ParseString(configContent)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ID = uuid.New().String()
	cfg.OwnerID = ownerID
	if name != "" {
		cfg.Name = name
	} else if cfg.Name == "" {
		cfg.Name = "tunnel-" + cfg.ID[:8]
	}

	t := NewTunnel(cfg)

	m.mu.Lock()
	m.tunnels[cfg.ID] = t
	m.mu.Unlock()

	// Intentionally no persistConfig call
	info := t.Info()
	m.emitChanged(info)
	return &info, nil
}

func (m *TunnelManager) DeleteTunnel(principal ipc.Principal, id string) error {
	ownerID := ownerForPrincipal(principal)
	m.mu.Lock()
	t, ok := m.tunnels[id]
	if !ok || !m.tunnelVisibleToOwner(t, ownerID) {
		m.mu.Unlock()
		return fmt.Errorf("tunnel %s not found", id)
	}
	delete(m.tunnels, id)
	m.mu.Unlock()

	_ = t.Stop()
	_ = m.deletePersistedConfig(t.Config.OwnerID, id)
	return nil
}

func (m *TunnelManager) ConnectTunnel(principal ipc.Principal, id string) error {
	ownerID := ownerForPrincipal(principal)
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok || !m.tunnelVisibleToOwner(t, ownerID) {
		return fmt.Errorf("tunnel %s not found", id)
	}

	go func() {
		info := t.Info()
		info.Status = ipc.StatusConnecting
		m.emitChanged(info)

		if err := t.Start(); err != nil {
			log.Printf("tunnel %s start error: %v", id, err)
			info = t.Info()
			info.Status = ipc.StatusError
			info.Error = err.Error()
			m.emitChanged(info)
			return
		}

		m.emitChanged(t.Info())
		m.startStatsLoop(t)
	}()

	return nil
}

func (m *TunnelManager) DisconnectTunnel(principal ipc.Principal, id string) error {
	ownerID := ownerForPrincipal(principal)
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok || !m.tunnelVisibleToOwner(t, ownerID) {
		return fmt.Errorf("tunnel %s not found", id)
	}

	if err := t.Stop(); err != nil {
		return err
	}

	m.emitChanged(t.Info())
	return nil
}

func (m *TunnelManager) GetStats(principal ipc.Principal, id string) (*ipc.StatsInfo, error) {
	ownerID := ownerForPrincipal(principal)
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok || !m.tunnelVisibleToOwner(t, ownerID) {
		return nil, fmt.Errorf("tunnel %s not found", id)
	}
	stats, err := t.Stats()
	if err != nil {
		return nil, err
	}
	return &ipc.StatsInfo{
		TunnelID:      id,
		OwnerID:       t.Config.OwnerID,
		RxBytes:       stats.RxBytes,
		TxBytes:       stats.TxBytes,
		LastHandshake: stats.LastHandshake,
	}, nil
}

func (m *TunnelManager) DaemonStatus() (*ipc.StatusResult, error) {
	return &ipc.StatusResult{
		Running:       true,
		DaemonVersion: "0.1.0",
	}, nil
}

// SetEncryptionKey stores the 32-byte AES-256 key in memory, then reloads
// persisted configs: decrypts any encrypted files, and migrates any remaining
// plaintext configs to encrypted format.
func (m *TunnelManager) SetEncryptionKey(principal ipc.Principal, key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	ownerID := ownerForPrincipal(principal)
	m.keyMu.Lock()
	m.encKeys[ownerID] = key
	m.keyMu.Unlock()

	if err := m.loadPersistedConfigs(ownerID); err != nil {
		log.Printf("warn: reload configs after key set: %v", err)
	}
	return nil
}

// StopAll gracefully stops all running tunnels. Called on daemon shutdown.
func (m *TunnelManager) StopAll() {
	m.mu.RLock()
	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		tunnels = append(tunnels, t)
	}
	m.mu.RUnlock()

	for _, t := range tunnels {
		if err := t.Stop(); err != nil {
			log.Printf("stop tunnel %s: %v", t.Config.ID, err)
		}
	}
}

// --- internal helpers ---

func (m *TunnelManager) emitChanged(info ipc.TunnelInfo) {
	if m.broadcast == nil {
		return
	}
	data, _ := json.Marshal(info)
	m.broadcast(ipc.Event{Type: ipc.EventTunnelChanged, Payload: data})
}

func (m *TunnelManager) startStatsLoop(t *Tunnel) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if t.Status() != ipc.StatusConnected {
				return
			}
			stats, err := t.Stats()
			if err != nil {
				return
			}
			if m.broadcast == nil {
				continue
			}
			info := ipc.StatsInfo{
				TunnelID:      t.Config.ID,
				OwnerID:       t.Config.OwnerID,
				RxBytes:       stats.RxBytes,
				TxBytes:       stats.TxBytes,
				LastHandshake: stats.LastHandshake,
			}
			data, _ := json.Marshal(info)
			m.broadcast(ipc.Event{Type: ipc.EventStatsUpdate, Payload: data})
		}
	}()
}

func ownerForPrincipal(principal ipc.Principal) string {
	if principal.Valid() {
		return principal.UserID
	}
	return ipc.LegacyOwnerID
}

func (m *TunnelManager) tunnelVisibleToOwner(t *Tunnel, ownerID string) bool {
	if t == nil || t.Config == nil {
		return false
	}
	return t.Config.OwnerID == ownerID || t.Config.OwnerID == ipc.LegacyOwnerID
}

func (m *TunnelManager) ownerStorageDir(ownerID string) string {
	if ownerID == "" || ownerID == ipc.LegacyOwnerID {
		return m.storageDir
	}
	return filepath.Join(m.storageDir, "users", safeOwnerID(ownerID))
}

func safeOwnerID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return ipc.LegacyOwnerID
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, ownerID)
}

// --- persistence ---

type persistedTunnel struct {
	ID        string                 `json:"id"`
	OwnerID   string                 `json:"owner_id,omitempty"`
	Name      string                 `json:"name"`
	Interface config.InterfaceConfig `json:"interface"`
	Peers     []config.PeerConfig    `json:"peers"`
}

func (m *TunnelManager) persistConfig(cfg *config.TunnelConfig) error {
	pt := persistedTunnel{
		ID:        cfg.ID,
		OwnerID:   cfg.OwnerID,
		Name:      cfg.Name,
		Interface: cfg.Interface,
		Peers:     cfg.Peers,
	}
	plain, err := json.MarshalIndent(pt, "", "  ")
	if err != nil {
		return err
	}

	m.keyMu.RLock()
	key := m.encKeys[cfg.OwnerID]
	m.keyMu.RUnlock()
	if key == nil {
		return fmt.Errorf("persistent tunnel storage encryption key is not set")
	}

	var toWrite []byte
	toWrite, err = encryptAES256GCM(key, plain)
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}

	dir := m.ownerStorageDir(cfg.OwnerID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create owner storage dir: %w", err)
	}
	if err := hardenStorageACL(dir); err != nil {
		log.Printf("warn: harden storage ACL %s: %v", dir, err)
	}
	path := filepath.Join(dir, cfg.ID+".json")
	return os.WriteFile(path, toWrite, 0600)
}

func (m *TunnelManager) deletePersistedConfig(ownerID, id string) error {
	path := filepath.Join(m.ownerStorageDir(ownerID), id+".json")
	return os.Remove(path)
}

func (m *TunnelManager) loadPersistedConfigs(ownerID string) error {
	m.keyMu.RLock()
	key := m.encKeys[ownerID]
	m.keyMu.RUnlock()
	if key == nil {
		return nil
	}

	if err := m.loadPersistedConfigDir(ownerID, m.ownerStorageDir(ownerID), key, false); err != nil {
		return err
	}
	if ownerID != ipc.LegacyOwnerID {
		if err := m.loadPersistedConfigDir(ownerID, m.storageDir, key, true); err != nil {
			log.Printf("warn: migrate legacy configs for owner %s: %v", ownerID, err)
		}
	}
	return nil
}

func (m *TunnelManager) loadPersistedConfigDir(ownerID, dir string, key []byte, migrateLegacy bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("read config %s: %v", e.Name(), err)
			continue
		}

		// Detect format: plaintext JSON starts with '{', otherwise treat as encrypted.
		isPlaintext := len(data) > 0 && data[0] == '{'

		var jsonData []byte
		if isPlaintext {
			if key == nil {
				// Legacy plaintext files are only read when they can be migrated
				// immediately to encrypted storage.
				continue
			}
			jsonData = data
		} else {
			if key == nil {
				// Encrypted but no key yet — skip until key is set.
				continue
			}
			plain, err := decryptAES256GCM(key, data)
			if err != nil {
				log.Printf("decrypt config %s: %v (skipping)", e.Name(), err)
				continue
			}
			jsonData = plain
		}

		var pt persistedTunnel
		if err := json.Unmarshal(jsonData, &pt); err != nil {
			log.Printf("parse config %s: %v", e.Name(), err)
			continue
		}
		if pt.OwnerID != "" && pt.OwnerID != ownerID && !migrateLegacy {
			log.Printf("skip config %s: owner mismatch", e.Name())
			continue
		}
		cfg := &config.TunnelConfig{
			ID:        pt.ID,
			OwnerID:   ownerID,
			Name:      pt.Name,
			Interface: pt.Interface,
			Peers:     pt.Peers,
		}

		m.mu.Lock()
		_, alreadyLoaded := m.tunnels[cfg.ID]
		migratedLoaded := false
		if !alreadyLoaded {
			m.tunnels[cfg.ID] = NewTunnel(cfg)
		} else if migrateLegacy && m.tunnels[cfg.ID].Config.OwnerID == ipc.LegacyOwnerID {
			m.tunnels[cfg.ID].Config.OwnerID = ownerID
			migratedLoaded = true
		}
		m.mu.Unlock()

		if alreadyLoaded && !migratedLoaded {
			continue
		}
		if !alreadyLoaded {
			log.Printf("loaded tunnel %s (%s)", cfg.Name, cfg.ID)
		}

		// Migrate plaintext or legacy global config to encrypted per-user storage.
		if isPlaintext || migrateLegacy {
			if err := m.persistConfig(cfg); err != nil {
				log.Printf("warn: migrate config %s to encrypted: %v", cfg.ID, err)
			} else {
				log.Printf("migrated tunnel %s to encrypted storage", cfg.ID)
				if migrateLegacy {
					_ = os.Remove(path)
				}
			}
		}
	}
	return nil
}

func (m *TunnelManager) encryptionKeySet(ownerID string) bool {
	m.keyMu.RLock()
	defer m.keyMu.RUnlock()
	return m.encKeys[ownerID] != nil
}
