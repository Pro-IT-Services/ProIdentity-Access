package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/curve25519"
	wgcrypto "wg-client/internal/crypto"
	"wg-client/internal/ipc"
	"wg-client/internal/managed"
	"wg-client/internal/secretstore"
)

// activeSession tracks one managed VPN session (one server connection).
type activeSession struct {
	sessionID string
	tunnelID  string
	stopKA    context.CancelFunc
}

// App is the Wails application struct.
type App struct {
	ctx    context.Context
	client *ipc.Client

	mMu                sync.Mutex
	mSettings          *managed.Settings
	mClient            *managed.Client
	mSessions          map[string]*activeSession // serverID → active session
	mConfigKey         []byte                    // user's server-side config key (cached in memory)
	mUserConfigs       map[string]string         // "uconf:{serverID}" → config name
	mUserConfigTunnels map[string]string         // "uconf:{serverID}" → ephemeral daemon tunnel ID (connected only)
	mPollCancel        context.CancelFunc        // cancels background poll loop
}

func NewApp() *App {
	settings, _ := managed.LoadSettings()
	if settings == nil {
		settings = &managed.Settings{}
	}
	a := &App{
		client:             ipc.NewClient(),
		mSettings:          settings,
		mSessions:          make(map[string]*activeSession),
		mUserConfigs:       make(map[string]string),
		mUserConfigTunnels: make(map[string]string),
	}
	// Restore encrypted client if device is registered and token exists.
	// NEVER fall back to unencrypted client when a device ID is present.
	if settings.ServerURL != "" && settings.Token != "" {
		if settings.DeviceID != "" && settings.ClientPrivateKey != "" && settings.ServerPublicKey != "" {
			ec, err := managed.NewEncryptedClient(settings.ServerURL, settings.Token, settings.DeviceID, settings.ClientPrivateKey, settings.ServerPublicKey)
			if err != nil {
				log.Printf("warn: could not restore encrypted client (corrupt registration?): %v", err)
			} else {
				a.mClient = ec
			}
		}
		// Do NOT create unencrypted client when device registration data is present.
	}
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tryConnect()
	go a.forwardDaemonEvents()
}

func (a *App) domReady(ctx context.Context) {
	a.setupTray(ctx)
}

// forwardDaemonEvents reads events from the IPC client and emits them to the Wails frontend.
// Daemon tunnel IDs for user-config tunnels are translated to "uconf:X" IDs.
func (a *App) forwardDaemonEvents() {
	for evt := range a.client.Events() {
		switch evt.Type {
		case ipc.EventStatsUpdate:
			var stats ipc.StatsInfo
			if json.Unmarshal(evt.Payload, &stats) == nil {
				// Translate daemon tunnel ID → uconf key if applicable
				if uconfKey := a.uconfKeyByTunnelID(stats.TunnelID); uconfKey != "" {
					stats.TunnelID = uconfKey
				}
				runtime.EventsEmit(a.ctx, evt.Type, stats)
			}
		case ipc.EventTunnelChanged:
			var info ipc.TunnelInfo
			if json.Unmarshal(evt.Payload, &info) == nil {
				if uconfKey := a.uconfKeyByTunnelID(info.ID); uconfKey != "" {
					info.ID = uconfKey
				}
				sanitizeTunnelInfo(&info)
				runtime.EventsEmit(a.ctx, evt.Type, info)
				signalTrayRefresh()
			}
		}
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.teardownTray()
	a.disconnectAll()
	a.client.Close()
}

func (a *App) tryConnect() {
	go func() {
		for {
			if err := a.client.Connect(); err == nil {
				log.Println("Connected to daemon")
				a.sendEncryptionKey()
				signalTrayRefresh()
				// Clear stale user-config tunnel IDs — daemon restarted, ephemeral tunnels are gone.
				a.mMu.Lock()
				a.mUserConfigTunnels = make(map[string]string)
				mc := a.mClient
				a.mMu.Unlock()
				if mc != nil {
					go a.syncServerConfigs(mc)
					a.startPollLoop(mc)
				}
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

// loadOrCreateEncryptionKey returns the user-level 32-byte AES-256 key used to
// encrypt tunnels stored permanently in the daemon (standalone mode only).
func (a *App) loadOrCreateEncryptionKey() ([]byte, error) {
	if data, err := secretstore.Get(secretstore.DaemonConfigKey); err == nil && len(data) == 32 {
		return data, nil
	} else if err != nil && !errors.Is(err, secretstore.ErrNotFound) {
		return nil, fmt.Errorf("read protected daemon config key: %w", err)
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("user config dir: %w", err)
	}
	keyDir := filepath.Join(dir, "ProIdentity")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}
	keyPath := filepath.Join(keyDir, "key.bin")

	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) == 32 {
		if err := secretstore.Put(secretstore.DaemonConfigKey, data); err != nil {
			return nil, fmt.Errorf("migrate daemon config key to protected storage: %w", err)
		}
		if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
			log.Printf("warn: remove migrated plaintext daemon key %s: %v", keyPath, err)
		}
		return data, nil
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := secretstore.Put(secretstore.DaemonConfigKey, key); err != nil {
		return nil, fmt.Errorf("write protected daemon config key: %w", err)
	}
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		log.Printf("warn: remove stale plaintext daemon key %s: %v", keyPath, err)
	}
	log.Println("Generated new protected encryption key for offline config storage")
	return key, nil
}

func (a *App) sendEncryptionKey() {
	if err := a.ensureDaemonEncryptionKey(); err != nil {
		log.Printf("warn: set encryption key on daemon: %v", err)
	}
}

func (a *App) ensureDaemonEncryptionKey() error {
	key, err := a.loadOrCreateEncryptionKey()
	if err != nil {
		return fmt.Errorf("encryption key: %w", err)
	}
	if err := a.client.SetEncryptionKey(key); err != nil {
		return err
	}
	return nil
}

// --- Standard tunnel methods ---

func (a *App) IsDaemonRunning() bool {
	return a.client.IsConnected()
}

func (a *App) ListTunnels() ([]ipc.TunnelInfo, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}
	tunnels, err := a.client.ListTunnels()
	if err != nil {
		return nil, err
	}

	a.mMu.Lock()
	// Build set of managed-session tunnel IDs (mark them is_managed=true)
	managedIDs := make(map[string]bool)
	for _, sess := range a.mSessions {
		if sess.tunnelID != "" {
			managedIDs[sess.tunnelID] = true
		}
	}
	// Build reverse map: daemonTunnelID → uconfKey (for connected user configs)
	daemonToUconf := make(map[string]string, len(a.mUserConfigTunnels))
	for uk, tID := range a.mUserConfigTunnels {
		daemonToUconf[tID] = uk
	}
	// Snapshot user configs
	userConfigs := make(map[string]string, len(a.mUserConfigs))
	for k, v := range a.mUserConfigs {
		userConfigs[k] = v
	}
	userConfigTunnels := make(map[string]string, len(a.mUserConfigTunnels))
	for k, v := range a.mUserConfigTunnels {
		userConfigTunnels[k] = v
	}
	mc := a.mClient
	a.mMu.Unlock()

	// Translate daemon tunnels: replace daemon IDs for user-config tunnels with "uconf:X"
	connectedUconfs := make(map[string]bool) // uconfKeys that are connected
	for i := range tunnels {
		if managedIDs[tunnels[i].ID] {
			tunnels[i].IsManaged = true
		}
		if uk, ok := daemonToUconf[tunnels[i].ID]; ok {
			tunnels[i].ID = uk
			connectedUconfs[uk] = true
		}
	}

	// In managed mode, add stubs for unconnected user configs
	if mc != nil {
		for uk, name := range userConfigs {
			if !connectedUconfs[uk] {
				tunnels = append(tunnels, ipc.TunnelInfo{
					ID:     uk,
					Name:   name,
					Status: ipc.StatusDisconnected,
				})
			}
		}
	}

	sanitizeTunnelInfos(tunnels)
	return tunnels, nil
}

// ImportTunnel imports a WireGuard config. In managed mode the config is uploaded
// to the server (encrypted) and stored only in memory — never written to disk.
// In standalone mode it is stored permanently in the daemon.
func (a *App) ImportTunnel(name, configContent string) (*ipc.TunnelInfo, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}

	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()

	if mc != nil {
		// Managed mode: upload to server, do NOT store in daemon
		key, err := a.getOrFetchConfigKey(mc)
		if err != nil {
			return nil, fmt.Errorf("get config key: %w", err)
		}
		encrypted, err := wgcrypto.EncryptAES256GCM(key, []byte(configContent))
		if err != nil {
			return nil, fmt.Errorf("encrypt config: %w", err)
		}
		serverID, err := mc.UploadUserConfig(name, encrypted)
		if err != nil {
			if handled := a.handleManagedAuthError(err); handled != nil {
				return nil, handled
			}
			return nil, fmt.Errorf("upload config to server: %w", err)
		}
		uconfKey := "uconf:" + serverID
		a.mMu.Lock()
		a.mUserConfigs[uconfKey] = name
		a.mMu.Unlock()
		log.Printf("imported config %q → server (id=%s)", name, serverID)
		return &ipc.TunnelInfo{
			ID:     uconfKey,
			Name:   name,
			Status: ipc.StatusDisconnected,
		}, nil
	}

	// Standalone mode: store permanently in daemon
	if err := a.ensureDaemonEncryptionKey(); err != nil {
		return nil, err
	}
	tunnel, err := a.client.ImportTunnel(name, configContent)
	if err != nil {
		return nil, err
	}
	return sanitizeTunnelInfo(tunnel), nil
}

func (a *App) DeleteTunnel(id string) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}

	if strings.HasPrefix(id, "uconf:") {
		serverID := strings.TrimPrefix(id, "uconf:")
		a.mMu.Lock()
		tID := a.mUserConfigTunnels[id]
		delete(a.mUserConfigs, id)
		delete(a.mUserConfigTunnels, id)
		mc := a.mClient
		a.mMu.Unlock()
		// Disconnect and delete ephemeral tunnel if connected
		if tID != "" {
			a.client.DisconnectTunnel(tID)
			a.client.DeleteTunnel(tID)
		}
		// Delete from server
		if mc != nil {
			go func() {
				if err := mc.DeleteUserConfig(serverID); err != nil {
					if a.handleManagedAuthError(err) != nil {
						return
					}
					log.Printf("warn: delete server config %s: %v", serverID, err)
				}
			}()
		}
		return nil
	}

	return a.client.DeleteTunnel(id)
}

// ConnectTunnel connects a tunnel. For "uconf:X" IDs (server-stored configs),
// it fetches the config from the server, imports it ephemerally, then connects.
func (a *App) ConnectTunnel(id string) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}

	if strings.HasPrefix(id, "uconf:") {
		return a.connectUserConfig(id)
	}

	return a.client.ConnectTunnel(id)
}

// connectUserConfig fetches a user config from the server, imports it ephemerally, and connects it.
func (a *App) connectUserConfig(uconfKey string) error {
	serverID := strings.TrimPrefix(uconfKey, "uconf:")

	a.mMu.Lock()
	mc := a.mClient
	name := a.mUserConfigs[uconfKey]
	// Disconnect old tunnel if any
	if oldTID := a.mUserConfigTunnels[uconfKey]; oldTID != "" {
		delete(a.mUserConfigTunnels, uconfKey)
		a.mMu.Unlock()
		a.client.DisconnectTunnel(oldTID)
		a.client.DeleteTunnel(oldTID)
	} else {
		a.mMu.Unlock()
	}

	if mc == nil {
		return fmt.Errorf("not logged in")
	}

	// Fetch config encryption key
	key, err := a.getOrFetchConfigKey(mc)
	if err != nil {
		return fmt.Errorf("get config key: %w", err)
	}

	// Download and decrypt config from server
	encrypted, err := mc.DownloadUserConfig(serverID)
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return handled
		}
		return fmt.Errorf("download config: %w", err)
	}
	plain, err := wgcrypto.DecryptAES256GCM(key, encrypted)
	if err != nil {
		return fmt.Errorf("decrypt config: %w", err)
	}

	// Import as ephemeral tunnel (not saved to disk)
	tunnel, err := a.client.ImportEphemeralTunnel(name, string(plain))
	if err != nil {
		return fmt.Errorf("import ephemeral tunnel: %w", err)
	}

	// Connect
	if err := a.client.ConnectTunnel(tunnel.ID); err != nil {
		a.client.DeleteTunnel(tunnel.ID)
		return fmt.Errorf("connect tunnel: %w", err)
	}

	a.mMu.Lock()
	a.mUserConfigTunnels[uconfKey] = tunnel.ID
	a.mMu.Unlock()

	log.Printf("connected user config %q (server=%s → daemon=%s)", name, serverID, tunnel.ID)
	return nil
}

// DisconnectTunnel disconnects a tunnel. For "uconf:X" IDs it also deletes the ephemeral tunnel.
func (a *App) DisconnectTunnel(id string) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}

	if strings.HasPrefix(id, "uconf:") {
		a.mMu.Lock()
		tID := a.mUserConfigTunnels[id]
		if tID != "" {
			delete(a.mUserConfigTunnels, id)
		}
		a.mMu.Unlock()
		if tID == "" {
			return nil
		}
		a.client.DisconnectTunnel(tID)
		a.client.DeleteTunnel(tID)
		return nil
	}

	return a.client.DisconnectTunnel(id)
}

func (a *App) GetStats(id string) (*ipc.StatsInfo, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}

	realID := id
	if strings.HasPrefix(id, "uconf:") {
		a.mMu.Lock()
		tID := a.mUserConfigTunnels[id]
		a.mMu.Unlock()
		if tID == "" {
			return nil, fmt.Errorf("tunnel not connected")
		}
		realID = tID
	}

	stats, err := a.client.GetStats(realID)
	if err != nil {
		return nil, err
	}
	// Translate back to uconf key for the frontend
	if realID != id {
		stats.TunnelID = id
	}
	return stats, nil
}

// --- Managed mode settings ---

// ManagedSettings is the data returned to the frontend.
type ManagedSettings struct {
	ServerURL   string `json:"server_url"`
	Username    string `json:"username"`
	IsAdmin     bool   `json:"is_admin"`
	LoggedIn    bool   `json:"logged_in"`
	VPNName     string `json:"vpn_name"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

// ManagedLoginResult is returned after a successful login.
type ManagedLoginResult struct {
	Username        string `json:"username"`
	IsAdmin         bool   `json:"is_admin"`
	RequireTOTP     bool   `json:"require_totp"`
	TOTPEnabled     bool   `json:"totp_enabled"`
	PushAuthEnabled bool   `json:"push_auth_enabled"`
	PushRequestID   string `json:"push_request_id,omitempty"`
}

// ManagedGetSettings returns the current managed-mode settings.
func (a *App) ManagedGetSettings() ManagedSettings {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	return ManagedSettings{
		ServerURL:   a.mSettings.ServerURL,
		Username:    a.mSettings.Username,
		IsAdmin:     a.mSettings.IsAdmin,
		LoggedIn:    a.mSettings.Token != "",
		VPNName:     a.mSettings.VPNName,
		TOTPEnabled: a.mSettings.TOTPEnabled,
	}
}

// ManagedSaveServerURL updates the management server URL and fetches the VPN name.
func (a *App) ManagedSaveServerURL(serverURL string) error {
	a.mMu.Lock()
	a.mSettings.ServerURL = serverURL
	if a.mClient != nil && a.mClient.BaseURL != serverURL {
		a.mSettings.Token = ""
		a.mSettings.Username = ""
		a.mSettings.IsAdmin = false
		a.mClient = nil
	}
	a.mMu.Unlock()

	c := managed.NewClient(serverURL, "")
	if info, err := c.GetInfo(); err == nil && info.VPNName != "" {
		a.mMu.Lock()
		a.mSettings.VPNName = info.VPNName
		a.mMu.Unlock()
	}

	a.mMu.Lock()
	defer a.mMu.Unlock()
	return a.mSettings.Save()
}

// ManagedLogin authenticates with the management server.
// If the device is registered, the login request is encrypted — no plain-text fallback.
func (a *App) ManagedLogin(username, password, totpCode string) (*ManagedLoginResult, error) {
	a.mMu.Lock()
	serverURL := a.mSettings.ServerURL
	deviceID := a.mSettings.DeviceID
	clientPrivKey := a.mSettings.ClientPrivateKey
	serverPubKey := a.mSettings.ServerPublicKey
	a.mMu.Unlock()

	if serverURL == "" {
		return nil, fmt.Errorf("server URL not configured")
	}

	var loginClient *managed.Client
	if deviceID != "" && clientPrivKey != "" && serverPubKey != "" {
		// Device is registered: login MUST be encrypted
		ec, err := managed.NewEncryptedClient(serverURL, "", deviceID, clientPrivKey, serverPubKey)
		if err != nil {
			return nil, fmt.Errorf("device registration corrupt, please re-register: %w", err)
		}
		loginClient = ec
	} else {
		loginClient = managed.NewClient(serverURL, "")
	}

	resp, err := loginClient.Login(username, password, totpCode)
	if err != nil {
		return nil, err
	}

	if resp.RequireTOTP {
		return &ManagedLoginResult{
			RequireTOTP:     true,
			PushAuthEnabled: resp.PushAuthEnabled,
			PushRequestID:   resp.PushRequestID,
		}, nil
	}

	a.mMu.Lock()
	a.mSettings.Token = resp.Token
	a.mSettings.Username = resp.Username
	a.mSettings.IsAdmin = resp.IsAdmin
	a.mSettings.TOTPEnabled = resp.TOTPEnabled
	if err := a.mSettings.Save(); err != nil {
		a.mMu.Unlock()
		return nil, err
	}

	var mc *managed.Client
	if a.mSettings.DeviceID != "" && a.mSettings.ClientPrivateKey != "" && a.mSettings.ServerPublicKey != "" {
		// Authenticated client MUST use encryption
		ec, err := managed.NewEncryptedClient(serverURL, resp.Token, a.mSettings.DeviceID, a.mSettings.ClientPrivateKey, a.mSettings.ServerPublicKey)
		if err != nil {
			a.mMu.Unlock()
			return nil, fmt.Errorf("could not create encrypted client: %w", err)
		}
		mc = ec
	}
	// mc is nil (unregistered device) — managed features unavailable
	a.mClient = mc
	a.mMu.Unlock()

	if mc != nil {
		go a.syncServerConfigs(mc)
		a.startPollLoop(mc)
	}

	return &ManagedLoginResult{
		Username:    resp.Username,
		IsAdmin:     resp.IsAdmin,
		TOTPEnabled: resp.TOTPEnabled,
	}, nil
}

// ManagedLoginWithPush completes login using an approved push auth request.
func (a *App) ManagedLoginWithPush(username, password, pushAuthID string) (*ManagedLoginResult, error) {
	a.mMu.Lock()
	serverURL := a.mSettings.ServerURL
	deviceID := a.mSettings.DeviceID
	clientPrivKey := a.mSettings.ClientPrivateKey
	serverPubKey := a.mSettings.ServerPublicKey
	a.mMu.Unlock()

	if serverURL == "" {
		return nil, fmt.Errorf("server URL not configured")
	}

	var loginClient *managed.Client
	if deviceID != "" && clientPrivKey != "" && serverPubKey != "" {
		ec, err := managed.NewEncryptedClient(serverURL, "", deviceID, clientPrivKey, serverPubKey)
		if err != nil {
			return nil, fmt.Errorf("device registration corrupt: %w", err)
		}
		loginClient = ec
	} else {
		loginClient = managed.NewClient(serverURL, "")
	}

	resp, err := loginClient.LoginWithPush(username, password, pushAuthID)
	if err != nil {
		return nil, err
	}

	a.mMu.Lock()
	a.mSettings.Token = resp.Token
	a.mSettings.Username = resp.Username
	a.mSettings.IsAdmin = resp.IsAdmin
	a.mSettings.TOTPEnabled = resp.TOTPEnabled
	if err := a.mSettings.Save(); err != nil {
		a.mMu.Unlock()
		return nil, err
	}

	var mc *managed.Client
	if a.mSettings.DeviceID != "" && a.mSettings.ClientPrivateKey != "" && a.mSettings.ServerPublicKey != "" {
		ec, err := managed.NewEncryptedClient(serverURL, resp.Token, a.mSettings.DeviceID, a.mSettings.ClientPrivateKey, a.mSettings.ServerPublicKey)
		if err != nil {
			a.mMu.Unlock()
			return nil, fmt.Errorf("could not create encrypted client: %w", err)
		}
		mc = ec
	}
	a.mClient = mc
	a.mMu.Unlock()

	if mc != nil {
		go a.syncServerConfigs(mc)
		a.startPollLoop(mc)
	}

	return &ManagedLoginResult{
		Username:    resp.Username,
		IsAdmin:     resp.IsAdmin,
		TOTPEnabled: resp.TOTPEnabled,
	}, nil
}

// ManagedLogout clears the stored token and disconnects all active sessions.
func (a *App) ManagedLogout() error {
	a.disconnectAll()
	a.stopPollLoop()

	// Disconnect and delete all active user-config ephemeral tunnels
	a.mMu.Lock()
	for _, tID := range a.mUserConfigTunnels {
		if tID != "" {
			a.client.DisconnectTunnel(tID)
			a.client.DeleteTunnel(tID)
		}
	}
	a.mUserConfigs = make(map[string]string)
	a.mUserConfigTunnels = make(map[string]string)
	a.mConfigKey = nil
	a.mClient = nil
	a.mSettings.Token = ""
	a.mSettings.Username = ""
	a.mSettings.IsAdmin = false
	a.mSettings.TOTPEnabled = false
	settings := a.mSettings
	a.mMu.Unlock()
	return settings.Save()
}

// --- Managed mode multi-server connections ---

// ManagedListServers returns the WireGuard servers accessible to the logged-in user.
func (a *App) ManagedListServers() ([]managed.ServerInfo, error) {
	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()
	if mc == nil {
		return nil, fmt.Errorf("not logged in")
	}
	servers, err := mc.ListServers()
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return nil, handled
		}
		return nil, err
	}
	return servers, nil
}

// ManagedConnectServer connects to a specific WireGuard server by ID.
// ManagedCreatePushAuth initiates a push auth request for the current user.
func (a *App) ManagedCreatePushAuth(context string) (map[string]any, error) {
	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()
	if mc == nil {
		return nil, fmt.Errorf("not logged in")
	}
	resp, err := mc.CreatePushAuth(context)
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return nil, handled
		}
		return nil, err
	}
	return map[string]any{
		"request_id": resp.RequestID,
		"status":     resp.Status,
		"expires_at": resp.ExpiresAt,
	}, nil
}

// ManagedPollPushAuth polls the status of a push auth request.
func (a *App) ManagedPollPushAuth(requestID string) (string, error) {
	// Try authenticated client first (for connect-time push).
	a.mMu.Lock()
	mc := a.mClient
	serverURL := a.mSettings.ServerURL
	a.mMu.Unlock()

	if mc != nil {
		st, err := mc.PollPushAuth(requestID)
		if err == nil {
			return st.Status, nil
		}
		if handled := a.handleManagedAuthError(err); handled != nil {
			return "", handled
		}
	}
	// Fall back to unauthenticated client (for login-time push).
	if serverURL == "" {
		return "", fmt.Errorf("server URL not configured")
	}
	uc := managed.NewClient(serverURL, "")
	st, err := uc.PollPushStatus(requestID)
	if err != nil {
		return "", err
	}
	return st.Status, nil
}

func (a *App) ManagedConnectServer(serverID, serverName, totpCode string) (*ipc.TunnelInfo, error) {
	return a.managedConnectInternal(serverID, serverName, totpCode, "")
}

// ManagedConnectServerPush connects using an approved push auth request ID.
func (a *App) ManagedConnectServerPush(serverID, serverName, pushAuthID string) (*ipc.TunnelInfo, error) {
	return a.managedConnectInternal(serverID, serverName, "", pushAuthID)
}

func (a *App) managedConnectInternal(serverID, serverName, totpCode, pushAuthID string) (*ipc.TunnelInfo, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}

	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()

	if mc == nil {
		return nil, fmt.Errorf("not logged in")
	}

	// Disconnect existing session for this server if any
	a.ManagedDisconnectServer(serverID)

	// Generate WireGuard keypair
	privKey, pubKey, err := generateKeypair()
	if err != nil {
		return nil, fmt.Errorf("keypair generation failed: %w", err)
	}

	// Create session on management server
	var sess *managed.SessionResponse
	if pushAuthID != "" {
		sess, err = mc.CreateSessionWithPush(serverID, pubKey, pushAuthID)
	} else {
		sess, err = mc.CreateSession(serverID, pubKey, totpCode)
	}
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return nil, handled
		}
		return nil, fmt.Errorf("session creation failed: %w", err)
	}
	if sess.RequireTOTP || sess.WGConfig == "" {
		if sess.PushAuthEnabled {
			return nil, fmt.Errorf("require_push_auth")
		}
		return nil, fmt.Errorf("require_totp")
	}

	// Inject our private key into the config the server returned
	wgConfig := injectPrivateKey(sess.WGConfig, privKey)

	tunnelName := serverName
	if tunnelName == "" {
		tunnelName = sess.VPNName
		if tunnelName == "" {
			tunnelName = "Managed VPN"
		}
	}

	// Import as ephemeral tunnel (not persisted to disk)
	tunnel, err := a.client.ImportEphemeralTunnel(tunnelName, wgConfig)
	if err != nil {
		mc.DeleteSession(sess.SessionID)
		return nil, fmt.Errorf("tunnel import failed: %w", err)
	}

	if err := a.client.ConnectTunnel(tunnel.ID); err != nil {
		mc.DeleteSession(sess.SessionID)
		a.client.DeleteTunnel(tunnel.ID)
		return nil, fmt.Errorf("tunnel connect failed: %w", err)
	}

	kaCtx, cancel := context.WithCancel(context.Background())

	a.mMu.Lock()
	a.mSessions[serverID] = &activeSession{
		sessionID: sess.SessionID,
		tunnelID:  tunnel.ID,
		stopKA:    cancel,
	}
	a.mMu.Unlock()

	go a.keepaliveLoop(kaCtx, mc, serverID, sess.SessionID, tunnel.ID)

	tunnel.IsManaged = true
	return sanitizeTunnelInfo(tunnel), nil
}

// ManagedDisconnectServer tears down the session for a specific server.
// The tunnel is disconnected immediately; the server DELETE call is retried in the
// background until it succeeds so that a lost connection doesn't block the UI.
func (a *App) ManagedDisconnectServer(serverID string) error {
	a.mMu.Lock()
	sess, ok := a.mSessions[serverID]
	if ok {
		delete(a.mSessions, serverID)
		if sess.stopKA != nil {
			sess.stopKA()
		}
	}
	mc := a.mClient
	a.mMu.Unlock()

	if !ok {
		return nil
	}

	// Tear down the tunnel immediately regardless of server reachability.
	if sess.tunnelID != "" {
		a.client.DisconnectTunnel(sess.tunnelID)
		a.client.DeleteTunnel(sess.tunnelID)
	}

	// Notify the server in the background with retries.
	if sess.sessionID != "" && mc != nil {
		go retryDeleteSession(mc, sess.sessionID)
	}
	return nil
}

// ManagedActiveServers returns the server IDs that currently have active sessions.
func (a *App) ManagedActiveServers() []string {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	ids := make([]string, 0, len(a.mSessions))
	for id := range a.mSessions {
		ids = append(ids, id)
	}
	return ids
}

// trayManagedClient returns the managed client and a set of currently connected server IDs.
// The lock is held only long enough to snapshot state; no network calls are made.
func (a *App) trayManagedClient() (mc *managed.Client, connected map[string]bool) {
	a.mMu.Lock()
	mc = a.mClient
	connected = make(map[string]bool, len(a.mSessions))
	for id := range a.mSessions {
		connected[id] = true
	}
	a.mMu.Unlock()
	return
}

func (a *App) trayDaemonTunnelIDsHiddenFromStandalone() map[string]bool {
	a.mMu.Lock()
	defer a.mMu.Unlock()

	hidden := make(map[string]bool, len(a.mSessions)+len(a.mUserConfigTunnels))
	for _, sess := range a.mSessions {
		if sess != nil && sess.tunnelID != "" {
			hidden[sess.tunnelID] = true
		}
	}
	for _, tunnelID := range a.mUserConfigTunnels {
		if tunnelID != "" {
			hidden[tunnelID] = true
		}
	}
	return hidden
}

// ManagedDisconnectByTunnelID finds the managed session owning tunnelID and disconnects it.
func (a *App) ManagedDisconnectByTunnelID(tunnelID string) error {
	// Check user-config tunnels first
	if strings.HasPrefix(tunnelID, "uconf:") {
		return a.DisconnectTunnel(tunnelID)
	}
	// Check managed VPN sessions
	a.mMu.Lock()
	var serverID string
	for sid, sess := range a.mSessions {
		if sess.tunnelID == tunnelID {
			serverID = sid
			break
		}
	}
	a.mMu.Unlock()

	if serverID != "" {
		return a.ManagedDisconnectServer(serverID)
	}
	// Fallback: best-effort daemon-level disconnect
	a.client.DisconnectTunnel(tunnelID)
	a.client.DeleteTunnel(tunnelID)
	return nil
}

// --- Installation revocation ---

// handleRevoked is called when the server reports this installation as revoked.
// It wipes all credentials and local state, then notifies the frontend.
func (a *App) handleRevoked() {
	log.Println("Installation revoked by server — wiping all local credentials and state")

	a.stopPollLoop()
	a.disconnectAll()
	a.wipeAllDaemonTunnels()

	// Disconnect and delete all ephemeral user-config tunnels
	a.mMu.Lock()
	for _, tID := range a.mUserConfigTunnels {
		if tID != "" {
			a.client.DisconnectTunnel(tID)
			a.client.DeleteTunnel(tID)
		}
	}
	a.mUserConfigs = make(map[string]string)
	a.mUserConfigTunnels = make(map[string]string)
	a.mConfigKey = nil
	a.mClient = nil

	// Replace settings with a blank struct so in-memory state is clean
	a.mSettings = &managed.Settings{}
	a.mMu.Unlock()

	// Delete the settings file entirely — no stale data left on disk
	a.mSettings.Delete()

	// Notify the frontend so it can show the setup wizard
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "installation_revoked", nil)
	}
}

// --- Online config sync (metadata only — no daemon imports) ---

// syncServerConfigs fetches the list of user configs from the server and updates
// the in-memory mUserConfigs map. Config content is NEVER pre-fetched or stored on disk.
// Configs are only downloaded when the user explicitly connects to them.
func (a *App) handleAuthInvalid() {
	a.resetManagedInstallation("Login expired or authorization revoked")
}

func (a *App) handleManagedAuthError(err error) error {
	if errors.Is(err, managed.ErrDeviceRevoked) {
		a.handleRevoked()
		return err
	}
	if errors.Is(err, managed.ErrAuthInvalid) {
		a.handleAuthInvalid()
		return err
	}
	return nil
}

func (a *App) resetManagedInstallation(reason string) {
	log.Printf("%s: wiping all local credentials and state", reason)

	a.stopPollLoop()
	a.disconnectAll()
	a.wipeAllDaemonTunnels()

	a.mMu.Lock()
	for _, tID := range a.mUserConfigTunnels {
		if tID != "" {
			a.client.DisconnectTunnel(tID)
			a.client.DeleteTunnel(tID)
		}
	}
	a.mUserConfigs = make(map[string]string)
	a.mUserConfigTunnels = make(map[string]string)
	a.mConfigKey = nil
	a.mClient = nil
	a.mSettings = &managed.Settings{}
	a.mMu.Unlock()

	_ = a.mSettings.Delete()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "installation_revoked", nil)
	}
}

func (a *App) wipeAllDaemonTunnels() {
	if err := a.ensureConnected(); err != nil {
		log.Printf("warn: wipe daemon tunnels: %v", err)
		return
	}
	tunnels, err := a.client.ListTunnels()
	if err != nil {
		log.Printf("warn: list daemon tunnels for wipe: %v", err)
		return
	}
	for _, tunnel := range tunnels {
		a.client.DisconnectTunnel(tunnel.ID)
		a.client.DeleteTunnel(tunnel.ID)
	}
	if dir, err := os.UserConfigDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, "ProIdentity", "key.bin"))
	}
}

func (a *App) syncServerConfigs(mc *managed.Client) error {
	serverConfigs, err := mc.ListUserConfigs()
	if err != nil {
		return err
	}

	a.mMu.Lock()
	defer a.mMu.Unlock()

	// Build set of server IDs we know about
	serverIDSet := make(map[string]bool, len(serverConfigs))
	for _, cfg := range serverConfigs {
		key := "uconf:" + cfg.ID
		serverIDSet[key] = true
		if _, exists := a.mUserConfigs[key]; !exists {
			a.mUserConfigs[key] = cfg.Name
			log.Printf("sync: discovered config %q (id=%s)", cfg.Name, cfg.ID)
		}
	}

	// Remove configs that no longer exist on the server
	for key := range a.mUserConfigs {
		if !serverIDSet[key] {
			log.Printf("sync: removing deleted config (key=%s)", key)
			if tID := a.mUserConfigTunnels[key]; tID != "" {
				a.client.DisconnectTunnel(tID)
				a.client.DeleteTunnel(tID)
			}
			delete(a.mUserConfigs, key)
			delete(a.mUserConfigTunnels, key)
		}
	}

	return nil
}

// getOrFetchConfigKey returns the cached config key, fetching from server if needed.
func (a *App) getOrFetchConfigKey(mc *managed.Client) ([]byte, error) {
	a.mMu.Lock()
	key := a.mConfigKey
	a.mMu.Unlock()
	if len(key) == 32 {
		return key, nil
	}
	key, err := mc.GetUserConfigKey()
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return nil, handled
		}
		return nil, err
	}
	a.mMu.Lock()
	a.mConfigKey = key
	a.mMu.Unlock()
	return key, nil
}

// --- Setup wizard methods ---

// ManagedCheckSetup returns true if the setup wizard has been completed.
func (a *App) ManagedCheckSetup() bool {
	settings, err := managed.LoadSettings()
	if err != nil {
		return false
	}
	return settings.SetupDone
}

// ManagedGetSetupStep returns the current wizard step derived from persisted settings.
func (a *App) ManagedGetSetupStep() string {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	s := a.mSettings
	if s.SetupDone || s.Mode == "standalone" {
		return "done"
	}
	if s.DeviceID != "" {
		return "login"
	}
	if s.ServerURL != "" && s.Mode == "managed" {
		return "register"
	}
	if s.Mode == "managed" {
		return "server"
	}
	return "mode"
}

// ManagedDisconnectByTunnelID is also used from the frontend to disconnect a tunnel by its ID
// regardless of whether it's a managed VPN session or a user config.
// (duplicate stub removed — declared above)

// ManagedSetMode sets the app mode ("standalone" or "managed") and marks setup done for standalone.
func (a *App) ManagedSetMode(mode string) error {
	a.mMu.Lock()
	a.mSettings.Mode = mode
	if mode == "standalone" {
		a.mSettings.SetupDone = true
	}
	err := a.mSettings.Save()
	a.mMu.Unlock()
	return err
}

func (a *App) ManagedDefaultDeviceName() string {
	return defaultDeviceName()
}

// ManagedRegisterDevice registers this installation with the server.
func (a *App) ManagedRegisterDevice(serverURL, deviceName string) error {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		deviceName = defaultDeviceName()
	}
	privKey, pubKey, err := managed.GenerateX25519KeyPair()
	if err != nil {
		return fmt.Errorf("generate keys: %w", err)
	}
	client := managed.NewClient(serverURL, "")
	resp, err := client.Register(deviceName, pubKey)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	a.mMu.Lock()
	a.mSettings.ServerURL = serverURL
	a.mSettings.DeviceID = resp.DeviceID
	a.mSettings.ClientPrivateKey = privKey
	a.mSettings.ServerPublicKey = resp.ServerPublicKey
	a.mSettings.Mode = "managed"
	err = a.mSettings.Save()
	a.mMu.Unlock()
	return err
}

// ManagedCompleteSetup marks the wizard as done after successful login.
func (a *App) ManagedCompleteSetup() error {
	a.mMu.Lock()
	a.mSettings.SetupDone = true
	err := a.mSettings.Save()
	a.mMu.Unlock()
	return err
}

// --- Internal helpers ---

func (a *App) ensureConnected() error {
	if a.client.IsConnected() {
		return nil
	}
	return a.client.Connect()
}

// disconnectAll tears down every active managed session.
// Tunnels are torn down immediately; server DELETE calls are retried in background.
func (a *App) disconnectAll() {
	a.mMu.Lock()
	sessions := make(map[string]*activeSession, len(a.mSessions))
	for k, v := range a.mSessions {
		sessions[k] = v
	}
	a.mSessions = make(map[string]*activeSession)
	mc := a.mClient
	a.mMu.Unlock()

	for _, sess := range sessions {
		if sess.stopKA != nil {
			sess.stopKA()
		}
		if sess.tunnelID != "" {
			a.client.DisconnectTunnel(sess.tunnelID)
			a.client.DeleteTunnel(sess.tunnelID)
		}
		if sess.sessionID != "" && mc != nil {
			go retryDeleteSession(mc, sess.sessionID)
		}
	}
}

// retryDeleteSession calls mc.DeleteSession in a loop until it succeeds,
// backing off 5 seconds between attempts. Used so that a temporarily
// unreachable server does not block the UI on disconnect.
func retryDeleteSession(mc *managed.Client, sessionID string) {
	for {
		err := mc.DeleteSession(sessionID)
		if err == nil {
			log.Printf("session %s deleted on server", sessionID)
			return
		}
		if errors.Is(err, managed.ErrDeviceRevoked) || errors.Is(err, managed.ErrAuthInvalid) {
			return
		}
		log.Printf("delete session %s failed: %v — retrying in 5s", sessionID, err)
		time.Sleep(5 * time.Second)
	}
}

func (a *App) startPollLoop(mc *managed.Client) {
	a.mMu.Lock()
	if a.mPollCancel != nil {
		a.mPollCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mPollCancel = cancel
	a.mMu.Unlock()
	go a.pollLoop(ctx, mc)
}

func (a *App) stopPollLoop() {
	a.mMu.Lock()
	if a.mPollCancel != nil {
		a.mPollCancel()
		a.mPollCancel = nil
	}
	a.mMu.Unlock()
}

// pollLoop syncs server config metadata every 10 seconds and checks for revocation
// and token validity (catches password changes, user disabled, etc.).
func (a *App) pollLoop(ctx context.Context, mc *managed.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check token is still valid (detects password change, user disabled, etc.)
			if err := mc.CheckAuth(); err != nil {
				if handled := a.handleManagedAuthError(err); handled != nil {
					return
				}
				log.Printf("poll: auth check failed: %v — forcing re-login", err)
				return
			}

			if err := a.syncServerConfigs(mc); err != nil {
				if handled := a.handleManagedAuthError(err); handled != nil {
					return
				}
				log.Printf("poll: sync error: %v", err)
			}
			// Notify the frontend so it can refresh the server list.
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "servers.changed")
			}
		}
	}
}

// keepaliveLoop sends keepalives every 10 seconds for a single server session.
func (a *App) keepaliveLoop(ctx context.Context, mc *managed.Client, serverID, sessionID, tunnelID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := mc.Keepalive(sessionID); err != nil {
				if handled := a.handleManagedAuthError(err); handled != nil {
					return
				}
				log.Printf("Keepalive failed for session %s (server %s): %v — tearing down", sessionID, serverID, err)
				a.mMu.Lock()
				if s, ok := a.mSessions[serverID]; ok && s.sessionID == sessionID {
					delete(a.mSessions, serverID)
					if s.stopKA != nil {
						s.stopKA()
					}
				}
				a.mMu.Unlock()
				a.client.DisconnectTunnel(tunnelID)
				a.client.DeleteTunnel(tunnelID)
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "session.revoked", serverID)
				}
				return
			}
		}
	}
}

// uconfKeyByTunnelID returns the "uconf:X" key for a given daemon tunnel ID, or "".
func (a *App) uconfKeyByTunnelID(daemonTunnelID string) string {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	for uk, tID := range a.mUserConfigTunnels {
		if tID == daemonTunnelID {
			return uk
		}
	}
	return ""
}

// generateKeypair creates a Curve25519 keypair suitable for WireGuard.
func generateKeypair() (privKeyB64, pubKeyB64 string, err error) {
	var privKey [32]byte
	if _, err = rand.Read(privKey[:]); err != nil {
		return
	}
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64

	pubKey, e := curve25519.X25519(privKey[:], curve25519.Basepoint)
	if e != nil {
		err = e
		return
	}
	privKeyB64 = base64.StdEncoding.EncodeToString(privKey[:])
	pubKeyB64 = base64.StdEncoding.EncodeToString(pubKey)
	return
}

// injectPrivateKey inserts the PrivateKey line into the [Interface] section.
func injectPrivateKey(wgConfig, privKey string) string {
	lines := splitLines(wgConfig)
	result := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		result = append(result, line)
		if strings.ToLower(strings.TrimSpace(line)) == "[interface]" {
			result = append(result, "PrivateKey = "+privKey)
		}
	}
	return joinLines(result)
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func defaultDeviceName() string {
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		return strings.TrimSpace(hostname)
	}
	return "ProIdentity Desktop"
}

func sanitizeTunnelInfos(tunnels []ipc.TunnelInfo) {
	for i := range tunnels {
		sanitizeTunnelInfo(&tunnels[i])
	}
}

func sanitizeTunnelInfo(tunnel *ipc.TunnelInfo) *ipc.TunnelInfo {
	if tunnel == nil {
		return nil
	}
	tunnel.PrivateKey = ""
	for i := range tunnel.Peers {
		tunnel.Peers[i].PublicKey = ""
	}
	return tunnel
}
