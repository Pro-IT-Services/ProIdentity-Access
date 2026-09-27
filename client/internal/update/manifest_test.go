package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func signed(t *testing.T, priv ed25519.PrivateKey) Manifest {
	t.Helper()
	m := Manifest{
		Platform: "windows-amd64",
		Version:  "0.7.3",
		FileName: "ProIdentity-Access-0.7.3.msi",
		URL:      "/api/v1/client-updates/windows-amd64/ProIdentity-Access-0.7.3.msi",
		SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Size:     6_660_096,
	}
	m.Sign(priv)
	return m
}

func TestSignedManifestVerifies(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	m := signed(t, priv)
	if err := m.VerifyWith(pub); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestTamperedFieldsAreRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	cases := map[string]func(*Manifest){
		"version":   func(m *Manifest) { m.Version = "9.9.9" },
		"sha256":    func(m *Manifest) { m.SHA256 = "f" + m.SHA256[1:] },
		"size":      func(m *Manifest) { m.Size++ },
		"file":      func(m *Manifest) { m.FileName = "Other-0.7.3.msi" },
		"platform":  func(m *Manifest) { m.Platform = "darwin-arm64"; m.FileName = "ProIdentity-Access-0.7.3.pkg" },
		"mandatory": func(m *Manifest) { m.Mandatory = true },
	}
	for name, mutate := range cases {
		m := signed(t, priv)
		mutate(&m)
		if err := m.VerifyWith(pub); err == nil {
			t.Errorf("%s: tampered manifest accepted", name)
		}
	}
}

func TestOtherKeyIsRejected(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	m := signed(t, priv)
	if err := m.VerifyWith(otherPub); err == nil {
		t.Fatal("manifest signed by another key accepted")
	}
}

func TestUnsignedManifestIsRejected(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	m := signed(t, priv)
	m.Signature = ""
	if err := m.VerifyWith(pub); err == nil {
		t.Fatal("unsigned manifest accepted")
	}
}

func TestValidateRejectsUnsafeFileNames(t *testing.T) {
	for _, name := range []string{"../evil.msi", `..\evil.msi`, "a/b.msi", "x.exe", "setup.msi.exe", ".msi", "evil.pkg"} {
		m := Manifest{Platform: "windows-amd64", Version: "1.0.0", FileName: name,
			SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Size: 10}
		if err := m.Validate(); err == nil {
			t.Errorf("file name %q accepted", name)
		}
	}
}

func TestReleaseKeyIsConfigured(t *testing.T) {
	pub, err := base64.StdEncoding.DecodeString(ReleasePublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatal("ReleasePublicKey is not a valid Ed25519 public key")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.7.3", "0.7.2", true},
		{"0.10.0", "0.9.9", true},
		{"1.0.0", "0.99.99", true},
		{"0.7.2", "0.7.2", false},
		{"0.7.1", "0.7.2", false},
		{"0.7.2", "0.0.0-dev", true},
		{"v0.7.3", "0.7.2", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
