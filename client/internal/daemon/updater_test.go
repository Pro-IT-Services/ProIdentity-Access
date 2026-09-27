package daemon

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"wg-client/internal/ipc"
	"wg-client/internal/update"
)

type feed struct {
	manifest update.Manifest
	pkg      []byte
}

func newFeed(t *testing.T, version string, pkg []byte, priv ed25519.PrivateKey) *feed {
	t.Helper()
	sum := sha256.Sum256(pkg)
	ext := map[string]string{"windows": ".msi", "darwin": ".pkg"}[strings.SplitN(update.CurrentPlatform(), "-", 2)[0]]
	if ext == "" {
		t.Skip("updates are not supported on this platform")
	}
	name := "ProIdentity-Access-" + version + ext
	m := update.Manifest{
		Platform: update.CurrentPlatform(),
		Version:  version,
		FileName: name,
		URL:      "/api/v1/client-updates/" + update.CurrentPlatform() + "/" + name,
		SHA256:   hex.EncodeToString(sum[:]),
		Size:     int64(len(pkg)),
	}
	if priv != nil {
		m.Sign(priv)
	}
	return &feed{manifest: m, pkg: pkg}
}

func (f *feed) serve(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			json.NewEncoder(w).Encode(f.manifest)
		case strings.HasSuffix(r.URL.Path, f.manifest.FileName):
			w.Write(f.pkg)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type harness struct {
	u       *Updater
	mu      sync.Mutex
	ran     []string
	states  []ipc.UpdateState
	done    chan struct{}
	doneOne sync.Once
}

func newHarness(t *testing.T, pub ed25519.PublicKey) *harness {
	t.Helper()
	old := update.Version
	update.Version = "0.7.2"
	t.Cleanup(func() { update.Version = old })

	h := &harness{done: make(chan struct{})}
	h.u = NewUpdater(filepath.Join(t.TempDir(), "updates"), func(evt ipc.Event) {
		var st ipc.UpdateState
		json.Unmarshal(evt.Payload, &st)
		h.mu.Lock()
		h.states = append(h.states, st)
		h.mu.Unlock()
		if st.State == "failed" || st.State == "installing" {
			h.doneOne.Do(func() { close(h.done) })
		}
	}, func() []ipc.Principal { return []ipc.Principal{{UserID: "S-1-5-21-test"}} })
	h.u.verify = func(m *update.Manifest) error { return m.VerifyWith(pub) }
	h.u.prepare = func(dir string) error { return os.MkdirAll(dir, 0o700) }
	h.u.pause = 0
	h.u.run = func(pkg, version, dir string, _ []ipc.Principal, _ func(error)) error {
		h.mu.Lock()
		h.ran = append(h.ran, pkg)
		h.mu.Unlock()
		return nil
	}
	return h
}

func (h *harness) wait(t *testing.T) ipc.UpdateState {
	t.Helper()
	select {
	case <-h.done:
	case <-time.After(10 * time.Second):
		t.Fatal("update did not finish")
	}
	// Let the install goroutine publish its final state.
	time.Sleep(50 * time.Millisecond)
	return h.u.UpdateStatus()
}

func TestCheckReportsSignedNewerUpdate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), priv)
	h := newHarness(t, pub)

	st, err := h.u.CheckUpdate(ipc.Principal{}, f.serve(t).URL)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "available" || st.LatestVersion != "0.7.3" {
		t.Fatalf("got %+v, want available 0.7.3", st)
	}
}

func TestCheckTreatsOlderOrUnsignedOldAsUpToDate(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.0", []byte("installer"), nil) // unsigned, like pre-0.7.2 servers
	h := newHarness(t, pub)
	st, err := h.u.CheckUpdate(ipc.Principal{}, f.serve(t).URL)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "up_to_date" {
		t.Fatalf("got %q, want up_to_date", st.State)
	}
}

func TestCheckRejectsUnsignedNewerUpdate(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), nil)
	h := newHarness(t, pub)
	if _, err := h.u.CheckUpdate(ipc.Principal{}, f.serve(t).URL); err == nil {
		t.Fatal("unsigned update accepted")
	}
}

func TestInstallRunsVerifiedPackage(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	pkg := []byte(strings.Repeat("genuine installer ", 1000))
	f := newFeed(t, "0.7.3", pkg, priv)
	h := newHarness(t, pub)

	if err := h.u.InstallUpdate(ipc.Principal{Username: "alice"}, f.serve(t).URL); err != nil {
		t.Fatal(err)
	}
	if st := h.wait(t); st.State != "installing" {
		t.Fatalf("state %q (%s), want installing", st.State, st.Error)
	}
	if len(h.ran) != 1 {
		t.Fatalf("installer ran %d times", len(h.ran))
	}
	got, err := os.ReadFile(h.ran[0])
	if err != nil || string(got) != string(pkg) {
		t.Fatal("installer was not handed the downloaded package")
	}
	if filepath.Dir(h.ran[0]) != h.u.dir {
		t.Fatalf("package outside the protected folder: %s", h.ran[0])
	}
}

func TestInstallRejectsTamperedPackage(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("genuine installer"), priv)
	f.pkg = []byte("evil installer!!!") // same length, different bytes
	h := newHarness(t, pub)

	if err := h.u.InstallUpdate(ipc.Principal{}, f.serve(t).URL); err != nil {
		t.Fatal(err)
	}
	st := h.wait(t)
	if st.State != "failed" || !strings.Contains(st.Error, "checksum") {
		t.Fatalf("got %q %q, want checksum failure", st.State, st.Error)
	}
	if len(h.ran) != 0 {
		t.Fatal("installer ran for a tampered package")
	}
	if entries, _ := os.ReadDir(h.u.dir); len(entries) != 0 {
		t.Fatalf("tampered download left behind: %v", entries)
	}
}

func TestInstallRejectsForeignKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, attacker, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), attacker)
	h := newHarness(t, pub)

	if err := h.u.InstallUpdate(ipc.Principal{}, f.serve(t).URL); err != nil {
		t.Fatal(err)
	}
	if st := h.wait(t); st.State != "failed" || len(h.ran) != 0 {
		t.Fatalf("package signed with another key was installed (%q)", st.State)
	}
}

func TestInstallRejectsOtherPlatform(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), nil)
	if strings.HasPrefix(f.manifest.Platform, "windows") {
		f.manifest.Platform, f.manifest.FileName = "darwin-arm64", "ProIdentity-Access-0.7.3.pkg"
	} else {
		f.manifest.Platform, f.manifest.FileName = "windows-amd64", "ProIdentity-Access-0.7.3.msi"
	}
	f.manifest.Sign(priv)
	h := newHarness(t, pub)

	if err := h.u.InstallUpdate(ipc.Principal{}, f.serve(t).URL); err != nil {
		t.Fatal(err)
	}
	if st := h.wait(t); st.State != "failed" || len(h.ran) != 0 {
		t.Fatalf("package for another platform was installed (%q)", st.State)
	}
}

func TestServerURLMustBeHTTPS(t *testing.T) {
	for _, bad := range []string{"http://vpn.example.com", "ftp://x", "file:///c:/x", "", "not a url"} {
		if _, err := parseServerURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, ok := range []string{"https://vpn.example.com", "https://vpn.example.com/", "http://127.0.0.1:8080", "http://localhost:8080"} {
		if _, err := parseServerURL(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
}

func TestPackageURLCannotDowngradeToHTTP(t *testing.T) {
	if _, err := resolvePackageURL("https://vpn.example.com", "http://evil.example.com/x.msi"); err == nil {
		t.Fatal("plain-http package URL accepted")
	}
	u, err := resolvePackageURL("https://vpn.example.com", "/api/v1/client-updates/windows-amd64/x.msi")
	if err != nil || u.String() != "https://vpn.example.com/api/v1/client-updates/windows-amd64/x.msi" {
		t.Fatalf("relative URL resolved to %v (%v)", u, err)
	}
}
