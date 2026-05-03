package managed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func useTempSettingsHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestSettingsSaveDoesNotWriteSecretsToManagedJSON(t *testing.T) {
	useTempSettingsHome(t)

	s := &Settings{
		ServerURL:        "https://vpn.example.test",
		Token:            "secret-token-value",
		Username:         "alice",
		DeviceID:         "device-1",
		ClientPrivateKey: "client-private-key-value",
		ServerPublicKey:  "server-public-key-value",
		Mode:             "managed",
		SetupDone:        true,
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(settingsPath())
	if err != nil {
		t.Fatalf("ReadFile(managed.json) error = %v", err)
	}
	text := string(data)
	for _, secret := range []string{s.Token, s.ClientPrivateKey, s.ServerPublicKey} {
		if strings.Contains(text, secret) {
			t.Fatalf("managed.json contains secret value %q: %s", secret, text)
		}
	}
	for _, field := range []string{"token", "client_private_key", "server_public_key"} {
		if strings.Contains(text, field) {
			t.Fatalf("managed.json contains secret field %q: %s", field, text)
		}
	}
}

func TestLoadSettingsMigratesLegacyPlaintextSecrets(t *testing.T) {
	useTempSettingsHome(t)

	legacy := map[string]any{
		"server_url":         "https://vpn.example.test",
		"token":              "legacy-token-value",
		"username":           "alice",
		"device_id":          "device-1",
		"client_private_key": "legacy-client-private-key",
		"server_public_key":  "legacy-server-public-key",
		"mode":               "managed",
		"setup_done":         true,
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath()), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(settingsPath(), data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	s, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if s.Token != "legacy-token-value" || s.ClientPrivateKey != "legacy-client-private-key" || s.ServerPublicKey != "legacy-server-public-key" {
		t.Fatalf("LoadSettings() did not restore migrated secrets: %#v", s)
	}

	rewritten, err := os.ReadFile(settingsPath())
	if err != nil {
		t.Fatalf("ReadFile(rewritten managed.json) error = %v", err)
	}
	text := string(rewritten)
	for _, secret := range []string{s.Token, s.ClientPrivateKey, s.ServerPublicKey} {
		if strings.Contains(text, secret) {
			t.Fatalf("rewritten managed.json contains legacy secret %q: %s", secret, text)
		}
	}
}
