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

// Regression: the app quits when "installing" is announced, so the users to
// reopen it for must be captured before that, or nothing gets relaunched.
func TestInstallReopensAppForRequesterEvenAfterAppsQuit(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), priv)
	h := newHarness(t, pub)
	h.u.principals = func() []ipc.Principal { return nil } // every app already quit
	var got []ipc.Principal
	h.u.run = func(pkg, version, dir string, users []ipc.Principal, _ func(error)) error {
		got = users
		return nil
	}

	if err := h.u.InstallUpdate(ipc.Principal{UserID: "S-1-5-21-alice", Username: "alice"}, f.serve(t).URL); err != nil {
		t.Fatal(err)
	}
	h.wait(t)
	if len(got) != 1 || got[0].UserID != "S-1-5-21-alice" {
		t.Fatalf("app would be reopened for %v, want alice", got)
	}
}

func TestPeriodicCheckOpensAppForUsersWithoutIt(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), priv)
	h := newHarness(t, pub)
	h.u.principals = func() []ipc.Principal { return []ipc.Principal{{UserID: "running"}} }
	h.u.activeUsers = func() []string { return []string{"running", "idle", "snoozed"} }
	opened := make(chan string, 8)
	h.u.openApp = func(ids []string) { opened <- ids[0] }

	if _, err := h.u.CheckUpdate(ipc.Principal{}, f.serve(t).URL); err != nil { // remembers the server
		t.Fatal(err)
	}
	h.u.SnoozeUpdate(ipc.Principal{UserID: "snoozed"}, "0.7.3")

	h.u.periodicCheck()
	h.u.periodicCheck() // the same version is offered only once per boot
	time.Sleep(50 * time.Millisecond)
	close(opened)
	var got []string
	for id := range opened {
		got = append(got, id)
	}
	if len(got) != 1 || got[0] != "idle" {
		t.Fatalf("opened the app for %v, want only [idle]", got)
	}
}

func TestMandatoryUpdateIgnoresLater(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), nil)
	f.manifest.Mandatory = true
	f.manifest.Sign(priv)
	h := newHarness(t, pub)
	h.u.principals = func() []ipc.Principal { return nil }
	h.u.activeUsers = func() []string { return []string{"bob"} }
	opened := make(chan string, 2)
	h.u.openApp = func(ids []string) { opened <- ids[0] }

	h.u.CheckUpdate(ipc.Principal{}, f.serve(t).URL)
	h.u.SnoozeUpdate(ipc.Principal{UserID: "bob"}, "0.7.3")
	h.u.periodicCheck()
	select {
	case id := <-opened:
		if id != "bob" {
			t.Fatalf("opened for %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("mandatory update was not offered after Later")
	}
}

func TestUpdatePrefsSurviveRestart(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := newFeed(t, "0.7.3", []byte("installer"), priv)
	h := newHarness(t, pub)
	srv := f.serve(t)
	h.u.CheckUpdate(ipc.Principal{}, srv.URL)
	h.u.SnoozeUpdate(ipc.Principal{UserID: "carol"}, "0.7.3")

	again := NewUpdater(h.u.dir, nil, func() []ipc.Principal { return nil })
	if again.source != srv.URL {
		t.Fatalf("server not remembered: %q", again.source)
	}
	if !again.snoozed("carol", "0.7.3") || again.snoozed("carol", "0.7.4") {
		t.Fatal("Later not remembered per version")
	}
}

// An update installed by an older service may leave an empty reopen list;
// the new service then reopens the app for the signed-in users.
func TestResumeAfterUpdateFallsBackToSignedInUsers(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	h := newHarness(t, pub)
	os.MkdirAll(h.u.dir, 0o700)
	writeRelaunchMarker(h.u.dir, update.Version, nil)
	h.u.activeUsers = func() []string { return []string{"S-1-5-21-dave"} }
	opened := make(chan []string, 1)
	h.u.openApp = func(ids []string) { opened <- ids }

	h.u.ResumeAfterUpdate()
	select {
	case ids := <-opened:
		if len(ids) != 1 || ids[0] != "S-1-5-21-dave" {
			t.Fatalf("reopened for %v", ids)
		}
	case <-time.After(time.Second):
		t.Fatal("app not reopened after update")
	}
}
