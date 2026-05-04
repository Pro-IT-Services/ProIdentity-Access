package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"wg-client/internal/secretstore"
)

func useTempUserConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func legacyDaemonKeyPath(t *testing.T) string {
	t.Helper()
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir() error = %v", err)
	}
	return filepath.Join(dir, "ProIdentity", "key.bin")
}

func TestLoadOrCreateEncryptionKeyStoresProtectedKeyOnly(t *testing.T) {
	useTempUserConfig(t)

	app := &App{}
	key, err := app.loadOrCreateEncryptionKey()
	if err != nil {
		t.Fatalf("loadOrCreateEncryptionKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}
	if _, err := os.Stat(legacyDaemonKeyPath(t)); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext key.bin still exists, stat error = %v", err)
	}
	stored, err := secretstore.Get(secretstore.DaemonConfigKey)
	if err != nil {
		t.Fatalf("secretstore.Get(DaemonConfigKey) error = %v", err)
	}
	if !bytes.Equal(stored, key) {
		t.Fatal("protected daemon key does not match generated key")
	}
}

func TestLoadOrCreateEncryptionKeyMigratesLegacyKeyBin(t *testing.T) {
	useTempUserConfig(t)

	keyPath := legacyDaemonKeyPath(t)
	legacy := []byte("abcdefghijklmnopqrstuvwxyz123456")
	if len(legacy) != 32 {
		t.Fatalf("legacy test key length = %d", len(legacy))
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(keyPath, legacy, 0600); err != nil {
		t.Fatalf("WriteFile(key.bin) error = %v", err)
	}

	app := &App{}
	key, err := app.loadOrCreateEncryptionKey()
	if err != nil {
		t.Fatalf("loadOrCreateEncryptionKey() error = %v", err)
	}
	if !bytes.Equal(key, legacy) {
		t.Fatal("migrated key does not match legacy key.bin")
	}
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy plaintext key.bin still exists after migration, stat error = %v", err)
	}
}
