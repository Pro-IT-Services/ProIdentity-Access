package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"wg-client/internal/ipc"
	"wg-client/internal/secretstore"
)

// OpenVPNProfileView is the unified profile shape the frontend renders: both
// admin-assigned (fetched from the server) and locally-imported profiles.
type OpenVPNProfileView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"` // "assigned" | "local"
	// AutofillName is what a password manager (RoboForm) matches the login
	// against; the app puts it in the window title while connecting. The
	// admin can set it per profile; otherwise it's the profile name.
	AutofillName     string `json:"autofill_name"`
	RequiresTOTP     bool   `json:"requires_totp"`
	AuthUserPass     bool   `json:"auth_user_pass"`
	DevType          string `json:"dev_type"`
	AllowCustomIP    bool   `json:"allow_custom_ip"`
	CustomIP         string `json:"custom_ip,omitempty"`
	RememberedUser   string `json:"remembered_user,omitempty"`
	HasSavedPassword bool   `json:"has_saved_password"`
}

type localOVPNProfile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DevType       string `json:"dev_type"`
	AuthUserPass  bool   `json:"auth_user_pass"`
	AllowCustomIP bool   `json:"allow_custom_ip"`
	RequiresTOTP  bool   `json:"requires_totp"`
}

// secretstore key helpers (config held only for locally-imported profiles).
func ovpnCfgKey(id string) string  { return "openvpn-cfg-" + id }
func ovpnPwKey(id string) string   { return "openvpn-pw-" + id }
func ovpnUserKey(id string) string { return "openvpn-user-" + id }

func localOVPNIndexPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ProIdentity", "openvpn_local.json"), nil
}

func loadLocalOVPNIndex() []localOVPNProfile {
	path, err := localOVPNIndexPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []localOVPNProfile
	_ = json.Unmarshal(data, &out)
	return out
}

func saveLocalOVPNIndex(profiles []localOVPNProfile) error {
	path, err := localOVPNIndexPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(profiles, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func hasSecret(key string) bool {
	_, err := secretstore.Get(key)
	return err == nil
}

func rememberedUser(id string) string {
	if data, err := secretstore.Get(ovpnUserKey(id)); err == nil {
		return string(data)
	}
	return ""
}

func autofillName(set, name string) string {
	if s := strings.TrimSpace(set); s != "" {
		return s
	}
	return strings.TrimSpace(name)
}

// ManagedListOpenVPNProfiles returns admin-assigned + locally-imported profiles.
func (a *App) ManagedListOpenVPNProfiles() ([]OpenVPNProfileView, error) {
	views := []OpenVPNProfileView{}

	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()
	if mc != nil {
		if profs, err := mc.ListOpenVPNProfiles(); err == nil {
			for _, p := range profs {
				views = append(views, OpenVPNProfileView{
					ID: p.ID, Name: p.Name, Source: "assigned", AutofillName: autofillName(p.AutofillName, p.Name),
					RequiresTOTP: p.RequiresTOTP, AuthUserPass: p.AuthUserPass,
					DevType: p.DevType, AllowCustomIP: p.AllowCustomIP, CustomIP: p.CustomIP,
					RememberedUser: rememberedUser(p.ID), HasSavedPassword: hasSecret(ovpnPwKey(p.ID)),
				})
			}
		} else {
			_ = a.handleManagedAuthError(err) // fall through to local profiles
		}
	}

	for _, lp := range loadLocalOVPNIndex() {
		views = append(views, OpenVPNProfileView{
			ID: lp.ID, Name: lp.Name, Source: "local", AutofillName: autofillName("", lp.Name),
			RequiresTOTP: lp.RequiresTOTP, AuthUserPass: lp.AuthUserPass,
			DevType: lp.DevType, AllowCustomIP: lp.AllowCustomIP,
			RememberedUser: rememberedUser(lp.ID), HasSavedPassword: hasSecret(ovpnPwKey(lp.ID)),
		})
	}
	return views, nil
}

// ManagedListOpenVPNSessions returns the daemon's live OpenVPN sessions.
func (a *App) ManagedListOpenVPNSessions() ([]ipc.OpenVPNStatus, error) {
	if err := a.ensureConnected(); err != nil {
		return nil, err
	}
	return a.client.ListOpenVPN()
}

// ManagedConnectOpenVPN connects an OpenVPN profile. password may be empty when
// a saved password should be used; totp is appended to the password for
// profiles flagged requires_totp.
func (a *App) ManagedConnectOpenVPN(id, source, username, password, totp, customIP string, remember bool) (*ipc.OpenVPNStatus, error) {
	var cfg, name, devType string
	var requiresTOTP bool

	switch source {
	case "local":
		lp, ok := findLocalProfile(id)
		if !ok {
			return nil, fmt.Errorf("profile not found")
		}
		data, err := secretstore.Get(ovpnCfgKey(id))
		if err != nil {
			return nil, fmt.Errorf("stored profile unavailable: %w", err)
		}
		cfg, name, devType, requiresTOTP = string(data), lp.Name, lp.DevType, lp.RequiresTOTP
	default: // "assigned"
		a.mMu.Lock()
		mc := a.mClient
		a.mMu.Unlock()
		if mc == nil {
			return nil, fmt.Errorf("not logged in")
		}
		pc, err := mc.GetOpenVPNConfig(id)
		if err != nil {
			if handled := a.handleManagedAuthError(err); handled != nil {
				return nil, handled
			}
			return nil, err
		}
		cfg, name, devType, requiresTOTP = pc.Config, pc.Name, pc.DevType, pc.RequiresTOTP
		if customIP == "" {
			customIP = pc.CustomIP
		}
	}

	if username == "" {
		username = rememberedUser(id)
	}
	basePassword := password
	if basePassword == "" {
		if data, err := secretstore.Get(ovpnPwKey(id)); err == nil {
			basePassword = string(data)
		}
	}
	finalPassword := basePassword
	if requiresTOTP && strings.TrimSpace(totp) != "" {
		finalPassword = basePassword + strings.TrimSpace(totp)
	}

	if remember {
		if username != "" {
			_ = secretstore.Put(ovpnUserKey(id), []byte(username))
		}
		if basePassword != "" {
			_ = secretstore.Put(ovpnPwKey(id), []byte(basePassword))
		}
	}

	if err := a.ensureConnected(); err != nil {
		return nil, err
	}
	return a.client.ConnectOpenVPN(ipc.OpenVPNConnectParams{
		ID:       source + ":" + id,
		Name:     name,
		Config:   cfg,
		DevType:  devType,
		CustomIP: customIP,
		Username: username,
		Password: finalPassword,
	})
}

// ManagedDisconnectOpenVPN stops a running OpenVPN session by its session id
// (the "source:id" form returned in OpenVPNStatus.ID).
func (a *App) ManagedDisconnectOpenVPN(sessionID string) error {
	if err := a.ensureConnected(); err != nil {
		return err
	}
	return a.client.DisconnectOpenVPN(sessionID)
}

// ImportOpenVPNProfile stores a user-provided .ovpn locally (config kept in the
// OS keystore). requiresTotp reflects the user's choice for external TOTP.
func (a *App) ImportOpenVPNProfile(name, config string, requiresTotp bool) (*OpenVPNProfileView, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(config) == "" {
		return nil, fmt.Errorf("name and config are required")
	}
	devType, authUserPass := detectOVPNMeta(config)
	id := uuid.New().String()

	if err := secretstore.Put(ovpnCfgKey(id), []byte(config)); err != nil {
		return nil, fmt.Errorf("store profile: %w", err)
	}
	lp := localOVPNProfile{
		ID: id, Name: name, DevType: devType, AuthUserPass: authUserPass,
		AllowCustomIP: devType == "tap", RequiresTOTP: requiresTotp,
	}
	profiles := append(loadLocalOVPNIndex(), lp)
	if err := saveLocalOVPNIndex(profiles); err != nil {
		_ = secretstore.Delete(ovpnCfgKey(id))
		return nil, err
	}
	return &OpenVPNProfileView{
		ID: id, Name: name, Source: "local", AutofillName: autofillName("", name), RequiresTOTP: requiresTotp,
		AuthUserPass: authUserPass, DevType: devType, AllowCustomIP: lp.AllowCustomIP,
	}, nil
}

// DeleteLocalOpenVPNProfile removes a locally-imported profile and its secrets.
func (a *App) DeleteLocalOpenVPNProfile(id string) error {
	profiles := loadLocalOVPNIndex()
	out := profiles[:0]
	found := false
	for _, p := range profiles {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return fmt.Errorf("profile not found")
	}
	_ = secretstore.Delete(ovpnCfgKey(id))
	_ = secretstore.Delete(ovpnPwKey(id))
	_ = secretstore.Delete(ovpnUserKey(id))
	return saveLocalOVPNIndex(out)
}

// ForgetOpenVPNPassword clears a saved password/username for a profile.
func (a *App) ForgetOpenVPNPassword(id string) error {
	_ = secretstore.Delete(ovpnPwKey(id))
	_ = secretstore.Delete(ovpnUserKey(id))
	return nil
}

func findLocalProfile(id string) (localOVPNProfile, bool) {
	for _, p := range loadLocalOVPNIndex() {
		if p.ID == id {
			return p, true
		}
	}
	return localOVPNProfile{}, false
}

// detectOVPNMeta extracts dev type (tun/tap) and auth-user-pass presence from a
// .ovpn config. A "dev-type" directive overrides the "dev" inference.
func detectOVPNMeta(cfg string) (devType string, authUserPass bool) {
	devType = "tun"
	var fromDev, fromType string
	for _, raw := range strings.Split(cfg, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "<") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "dev":
			if len(f) > 1 {
				v := strings.ToLower(f[1])
				if strings.HasPrefix(v, "tap") {
					fromDev = "tap"
				} else if strings.HasPrefix(v, "tun") {
					fromDev = "tun"
				}
			}
		case "dev-type":
			if len(f) > 1 {
				if v := strings.ToLower(f[1]); v == "tap" || v == "tun" {
					fromType = v
				}
			}
		case "auth-user-pass":
			authUserPass = true
		}
	}
	switch {
	case fromType != "":
		devType = fromType
	case fromDev != "":
		devType = fromDev
	}
	return devType, authUserPass
}
