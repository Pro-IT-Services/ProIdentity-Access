package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wg-client/internal/ipc"
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

	principal := ipc.Principal{Platform: "test", UserID: "user-a", Username: "user-a"}
	if _, err := m.ImportTunnel(principal, "corp", testWireGuardConfig); err == nil {
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
	principal := ipc.Principal{Platform: "test", UserID: "user-a", Username: "user-a"}
	if err := m.SetEncryptionKey(principal, []byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("SetEncryptionKey() error = %v", err)
	}

	info, err := m.ImportTunnel(principal, "corp", testWireGuardConfig)
	if err != nil {
		t.Fatalf("ImportTunnel() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "users", "user-a", info.ID+".json"))
	if err != nil {
		t.Fatalf("ReadFile(persisted config) error = %v", err)
	}
	if strings.HasPrefix(string(data), "{") || strings.Contains(string(data), "test-private-key") {
		t.Fatalf("persisted config is plaintext: %q", string(data))
	}
}

func TestTunnelOperationsAreIsolatedByOwner(t *testing.T) {
	dir := t.TempDir()
	m, err := NewTunnelManager(dir, nil)
	if err != nil {
		t.Fatalf("NewTunnelManager() error = %v", err)
	}
	ownerA := ipc.Principal{Platform: "test", UserID: "user-a", Username: "user-a"}
	ownerB := ipc.Principal{Platform: "test", UserID: "user-b", Username: "user-b"}
	key := []byte("0123456789abcdef0123456789abcdef")
	if err := m.SetEncryptionKey(ownerA, key); err != nil {
		t.Fatalf("SetEncryptionKey(ownerA) error = %v", err)
	}
	if err := m.SetEncryptionKey(ownerB, key); err != nil {
		t.Fatalf("SetEncryptionKey(ownerB) error = %v", err)
	}

	info, err := m.ImportTunnel(ownerA, "corp", testWireGuardConfig)
	if err != nil {
		t.Fatalf("ImportTunnel(ownerA) error = %v", err)
	}
	if info.OwnerID != ownerA.UserID {
		t.Fatalf("ImportTunnel() owner = %q, want %q", info.OwnerID, ownerA.UserID)
	}

	listA, err := m.ListTunnels(ownerA)
	if err != nil {
		t.Fatalf("ListTunnels(ownerA) error = %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("ListTunnels(ownerA) len = %d, want 1", len(listA))
	}
	listB, err := m.ListTunnels(ownerB)
	if err != nil {
		t.Fatalf("ListTunnels(ownerB) error = %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("ListTunnels(ownerB) len = %d, want 0", len(listB))
	}
	if err := m.DeleteTunnel(ownerB, info.ID); err == nil {
		t.Fatal("DeleteTunnel(ownerB) deleted ownerA tunnel")
	}
	if err := m.DeleteTunnel(ownerA, info.ID); err != nil {
		t.Fatalf("DeleteTunnel(ownerA) error = %v", err)
	}
}
