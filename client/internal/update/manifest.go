// Package update defines the signed client-update manifest shared by the GUI,
// the daemon (which installs updates with system rights) and the release
// tooling that publishes them.
//
// Trust model: the daemon runs as LocalSystem / root, so it must never install
// a package just because someone handed it a URL. Every manifest is signed
// with the release Ed25519 key (private half kept off the repo, public half
// embedded below). The signature covers platform, version, file name, size and
// SHA-256, so a package is only installed if it is byte-for-byte the one the
// release build published, for this platform, and newer than what is running.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Version is the running build's version, stamped at build time:
//
//	-ldflags "-X wg-client/internal/update.Version=0.7.3"
var Version = "0.0.0-dev"

// ReleasePublicKey verifies update manifests (Ed25519, base64).
// Its private half is the release signing key; see tools/updatesign.
const ReleasePublicKey = "Xg9Cf8ERjAp2RSo5w+B1Dlm4YofHXSblYMiYQaBUn4c="

// Manifest describes one published client installer.
type Manifest struct {
	Platform    string `json:"platform"` // e.g. "windows-amd64", "darwin-arm64"
	Version     string `json:"version"`
	FileName    string `json:"filename"`
	URL         string `json:"url"` // absolute https URL, or path relative to the server
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	PublishedAt string `json:"published_at,omitempty"`
	Mandatory   bool   `json:"mandatory"`
	Notes       string `json:"notes,omitempty"`
	Signature   string `json:"signature"` // base64 Ed25519 over SigningPayload()

	// Kept for clients <= 0.7.0, which read latest_version.
	LatestVersion string `json:"latest_version,omitempty"`
}

// CurrentPlatform is the manifest platform id of the running binary.
func CurrentPlatform() string { return runtime.GOOS + "-" + runtime.GOARCH }

var (
	fileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	sha256Re   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionRe  = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// SigningPayload is the exact byte string that is signed. Changing its layout
// is a breaking change for every installed client.
func (m *Manifest) SigningPayload() []byte {
	return []byte(strings.Join([]string{
		"proidentity-access-update-v1",
		m.Platform,
		m.Version,
		m.FileName,
		strings.ToLower(m.SHA256),
		strconv.FormatInt(m.Size, 10),
		strconv.FormatBool(m.Mandatory),
	}, "\n"))
}

// Validate checks field formats (not the signature).
func (m *Manifest) Validate() error {
	switch {
	case !versionRe.MatchString(m.Version):
		return fmt.Errorf("invalid version %q", m.Version)
	case !fileNameRe.MatchString(m.FileName) || strings.Contains(m.FileName, ".."):
		return fmt.Errorf("invalid file name %q", m.FileName)
	case !sha256Re.MatchString(m.SHA256):
		return errors.New("invalid sha256")
	case m.Size <= 0 || m.Size > MaxPackageSize:
		return fmt.Errorf("invalid size %d", m.Size)
	}
	ext := strings.ToLower(m.FileName[strings.LastIndex(m.FileName, ".")+1:])
	want := map[string]string{"windows": "msi", "darwin": "pkg"}[strings.SplitN(m.Platform, "-", 2)[0]]
	if want == "" || ext != want {
		return fmt.Errorf("package %q does not match platform %q", m.FileName, m.Platform)
	}
	return nil
}

// MaxPackageSize bounds downloads (the installers are ~10–60 MB).
const MaxPackageSize = 512 << 20

// Sign sets Signature using the release private key.
func (m *Manifest) Sign(priv ed25519.PrivateKey) {
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, m.SigningPayload()))
}

// Verify checks format and signature against the embedded release key.
func (m *Manifest) Verify() error {
	pub, err := base64.StdEncoding.DecodeString(ReleasePublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("update signing key is not configured in this build")
	}
	return m.VerifyWith(ed25519.PublicKey(pub))
}

// VerifyWith checks format and signature against pub.
func (m *Manifest) VerifyWith(pub ed25519.PublicKey) error {
	if err := m.Validate(); err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("manifest is not signed")
	}
	if !ed25519.Verify(pub, m.SigningPayload(), sig) {
		return errors.New("manifest signature is invalid")
	}
	return nil
}

// Newer reports whether version a is greater than b (dotted numeric).
func Newer(a, b string) bool {
	ap, bp := parse(a), parse(b)
	for i := 0; i < len(ap) || i < len(bp); i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av != bv {
			return av > bv
		}
	}
	return false
}

func parse(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}
