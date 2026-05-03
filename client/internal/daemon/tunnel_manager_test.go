package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testWireGuardConfig = `[Interface]
PrivateKey = test-private-key
Address = 10.0.0.2/32

[Peer]
PublicKey = test-public-key
AllowedIPs = 0.0.0.0/0
`

func TestImportTunnelRequiresEncryptionKeyBeforePersisting(t *testing.T) {
	dir := t.TempDir()
	m, err := NewTunnelManager(dir, nil)
	if err != nil {
		t.Fatalf("NewTunnelManager() error = %v", err)
	}

	if _, err := m.ImportTunnel("corp", testWireGuardConfig); err == nil {
		t.Fatal("ImportTunnel() succeeded without encryption key")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ImportTunnel() wrote files without encryption key: %#v", entries)
	}
}

func TestImportTunnelPersistsEncryptedConfigWhenKeyIsSet(t *testing.T) {
	dir := t.TempDir()
	m, err := NewTunnelManager(dir, nil)
	if err != nil {
		t.Fatalf("NewTunnelManager() error = %v", err)
	}
	if err := m.SetEncryptionKey([]byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("SetEncryptionKey() error = %v", err)
	}

	info, err := m.ImportTunnel("corp", testWireGuardConfig)
	if err != nil {
		t.Fatalf("ImportTunnel() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, info.ID+".json"))
	if err != nil {
		t.Fatalf("ReadFile(persisted config) error = %v", err)
	}
	if strings.HasPrefix(string(data), "{") || strings.Contains(string(data), "test-private-key") {
		t.Fatalf("persisted config is plaintext: %q", string(data))
	}
}
