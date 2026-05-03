package managed

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"wg-client/internal/secretstore"
)

// Settings persists managed-mode configuration between app restarts. Secret
// fields are kept in the OS credential store and are never written to JSON.
type Settings struct {
	ServerURL   string `json:"server_url"`
	Token       string `json:"-"`
	Username    string `json:"username"`
	IsAdmin     bool   `json:"is_admin"`
	VPNName     string `json:"vpn_name"`
	TOTPEnabled bool   `json:"totp_enabled"`

	// Device registration fields
	DeviceID         string `json:"device_id,omitempty"`
	ClientPrivateKey string `json:"-"`              // base64 X25519 private key
	ServerPublicKey  string `json:"-"`              // base64 X25519 public key
	Mode             string `json:"mode,omitempty"` // "standalone" or "managed"
	SetupDone        bool   `json:"setup_done,omitempty"`
}

type settingsDisk struct {
	ServerURL   string `json:"server_url"`
	Token       string `json:"token,omitempty"` // legacy plaintext field; migrated on load
	Username    string `json:"username"`
	IsAdmin     bool   `json:"is_admin"`
	VPNName     string `json:"vpn_name"`
	TOTPEnabled bool   `json:"totp_enabled"`

	DeviceID         string `json:"device_id,omitempty"`
	ClientPrivateKey string `json:"client_private_key,omitempty"` // legacy plaintext field; migrated on load
	ServerPublicKey  string `json:"server_public_key,omitempty"`  // legacy plaintext field; migrated on load
	Mode             string `json:"mode,omitempty"`
	SetupDone        bool   `json:"setup_done,omitempty"`
}

func (d settingsDisk) toSettings() *Settings {
	return &Settings{
		ServerURL:        d.ServerURL,
		Token:            d.Token,
		Username:         d.Username,
		IsAdmin:          d.IsAdmin,
		VPNName:          d.VPNName,
		TOTPEnabled:      d.TOTPEnabled,
		DeviceID:         d.DeviceID,
		ClientPrivateKey: d.ClientPrivateKey,
		ServerPublicKey:  d.ServerPublicKey,
		Mode:             d.Mode,
		SetupDone:        d.SetupDone,
	}
}

func (d settingsDisk) hasLegacySecrets() bool {
	return d.Token != "" || d.ClientPrivateKey != "" || d.ServerPublicKey != ""
}

func (s *Settings) disk() settingsDisk {
	return settingsDisk{
		ServerURL:   s.ServerURL,
		Username:    s.Username,
		IsAdmin:     s.IsAdmin,
		VPNName:     s.VPNName,
		TOTPEnabled: s.TOTPEnabled,
		DeviceID:    s.DeviceID,
		Mode:        s.Mode,
		SetupDone:   s.SetupDone,
	}
}

func settingsPath() string {
	var dir string
	switch runtime.GOOS {
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"), "ProIdentity")
	case "darwin":
		dir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "ProIdentity")
	default:
		dir = filepath.Join(os.Getenv("HOME"), ".config", "proidentity")
	}
	return filepath.Join(dir, "managed.json")
}

func LoadSettings() (*Settings, error) {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{}, nil
		}
		return nil, err
	}
	var d settingsDisk
	if err := json.Unmarshal(data, &d); err != nil {
		return &Settings{}, nil
	}
	s := d.toSettings()
	if err := s.loadSecrets(); err != nil {
		return nil, err
	}
	if d.hasLegacySecrets() {
		if err := s.Save(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Delete removes the settings file and all managed secrets from disk.
func (s *Settings) Delete() error {
	_ = secretstore.Delete(secretstore.ManagedToken)
	_ = secretstore.Delete(secretstore.ManagedClientPrivKey)
	_ = secretstore.Delete(secretstore.ManagedServerPublicKey)
	err := os.Remove(settingsPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *Settings) Save() error {
	if err := s.saveSecrets(); err != nil {
		return err
	}
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.disk(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (s *Settings) loadSecrets() error {
	if err := loadSecretString(secretstore.ManagedToken, &s.Token); err != nil {
		return err
	}
	if err := loadSecretString(secretstore.ManagedClientPrivKey, &s.ClientPrivateKey); err != nil {
		return err
	}
	if err := loadSecretString(secretstore.ManagedServerPublicKey, &s.ServerPublicKey); err != nil {
		return err
	}
	return nil
}

func loadSecretString(name string, dest *string) error {
	value, err := secretstore.Get(name)
	if errors.Is(err, secretstore.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	*dest = string(value)
	return nil
}

func (s *Settings) saveSecrets() error {
	if err := putOrDeleteSecret(secretstore.ManagedToken, s.Token); err != nil {
		return err
	}
	if err := putOrDeleteSecret(secretstore.ManagedClientPrivKey, s.ClientPrivateKey); err != nil {
		return err
	}
	if err := putOrDeleteSecret(secretstore.ManagedServerPublicKey, s.ServerPublicKey); err != nil {
		return err
	}
	return nil
}

func putOrDeleteSecret(name, value string) error {
	if value == "" {
		return secretstore.Delete(name)
	}
	return secretstore.Put(name, []byte(value))
}
