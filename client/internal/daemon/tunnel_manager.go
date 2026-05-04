package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

	keyMu  sync.RWMutex
	encKey []byte // 32-byte AES-256 key; nil until set by GUI
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
	}
	if err := m.loadPersistedConfigs(); err != nil {
		log.Printf("warn: load configs: %v", err)
	}
	return m, nil
}

// --- ipc.Handler implementation ---

func (m *TunnelManager) ListTunnels() ([]ipc.TunnelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []ipc.TunnelInfo
	for _, t := range m.tunnels {
		list = append(list, t.Info())
	}
	return list, nil
}

func (m *TunnelManager) ImportTunnel(name, configContent string) (*ipc.TunnelInfo, error) {
	if !m.encryptionKeySet() {
		return nil, fmt.Errorf("persistent tunnel storage encryption key is not set")
	}

	cfg, err := config.ParseString(configContent)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ID = uuid.New().String()
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
func (m *TunnelManager) ImportEphemeralTunnel(name, configContent string) (*ipc.TunnelInfo, error) {
	cfg, err := config.ParseString(configContent)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.ID = uuid.New().String()
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

func (m *TunnelManager) DeleteTunnel(id string) error {
	m.mu.Lock()
	t, ok := m.tunnels[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("tunnel %s not found", id)
	}
	delete(m.tunnels, id)
	m.mu.Unlock()

	_ = t.Stop()
	_ = m.deletePersistedConfig(id)
	return nil
}

func (m *TunnelManager) ConnectTunnel(id string) error {
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok {
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

func (m *TunnelManager) DisconnectTunnel(id string) error {
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tunnel %s not found", id)
	}

	if err := t.Stop(); err != nil {
		return err
	}

	m.emitChanged(t.Info())
	return nil
}

func (m *TunnelManager) GetStats(id string) (*ipc.StatsInfo, error) {
	m.mu.RLock()
	t, ok := m.tunnels[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tunnel %s not found", id)
	}
	stats, err := t.Stats()
	if err != nil {
		return nil, err
	}
	return &ipc.StatsInfo{
		TunnelID:      id,
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
func (m *TunnelManager) SetEncryptionKey(key []byte) error {
	if len(key) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	m.keyMu.Lock()
	m.encKey = key
	m.keyMu.Unlock()

	if err := m.loadPersistedConfigs(); err != nil {
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
				RxBytes:       stats.RxBytes,
				TxBytes:       stats.TxBytes,
				LastHandshake: stats.LastHandshake,
			}
			data, _ := json.Marshal(info)
			m.broadcast(ipc.Event{Type: ipc.EventStatsUpdate, Payload: data})
		}
	}()
}

// --- persistence ---

type persistedTunnel struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Interface config.InterfaceConfig `json:"interface"`
	Peers     []config.PeerConfig    `json:"peers"`
}

func (m *TunnelManager) persistConfig(cfg *config.TunnelConfig) error {
	pt := persistedTunnel{
		ID:        cfg.ID,
		Name:      cfg.Name,
		Interface: cfg.Interface,
		Peers:     cfg.Peers,
	}
	plain, err := json.MarshalIndent(pt, "", "  ")
	if err != nil {
		return err
	}

	m.keyMu.RLock()
	key := m.encKey
	m.keyMu.RUnlock()
	if key == nil {
		return fmt.Errorf("persistent tunnel storage encryption key is not set")
	}

	var toWrite []byte
	toWrite, err = encryptAES256GCM(key, plain)
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}

	path := filepath.Join(m.storageDir, cfg.ID+".json")
	return os.WriteFile(path, toWrite, 0600)
}

func (m *TunnelManager) deletePersistedConfig(id string) error {
	path := filepath.Join(m.storageDir, id+".json")
	return os.Remove(path)
}

func (m *TunnelManager) loadPersistedConfigs() error {
	m.keyMu.RLock()
	key := m.encKey
	m.keyMu.RUnlock()

	entries, err := os.ReadDir(m.storageDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(m.storageDir, e.Name())
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
		cfg := &config.TunnelConfig{
			ID:        pt.ID,
			Name:      pt.Name,
			Interface: pt.Interface,
			Peers:     pt.Peers,
		}

		m.mu.Lock()
		_, alreadyLoaded := m.tunnels[cfg.ID]
		if !alreadyLoaded {
			m.tunnels[cfg.ID] = NewTunnel(cfg)
		}
		m.mu.Unlock()

		if alreadyLoaded {
			continue
		}
		log.Printf("loaded tunnel %s (%s)", cfg.Name, cfg.ID)

		// Migrate plaintext config to encrypted now that the key is available.
		if isPlaintext && key != nil {
			if err := m.persistConfig(cfg); err != nil {
				log.Printf("warn: migrate config %s to encrypted: %v", cfg.ID, err)
			} else {
				log.Printf("migrated tunnel %s to encrypted storage", cfg.ID)
			}
		}
	}
	return nil
}

func (m *TunnelManager) encryptionKeySet() bool {
	m.keyMu.RLock()
	defer m.keyMu.RUnlock()
	return m.encKey != nil
}
