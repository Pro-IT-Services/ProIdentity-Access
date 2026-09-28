package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wg-client/internal/ipc"
	"wg-client/internal/update"
)

// Updater downloads and installs client updates with the daemon's system
// rights, so users who can't install software can still update.
//
// Nothing the GUI sends is trusted: the GUI only names the management server.
// The daemon fetches the manifest itself, verifies its Ed25519 signature
// against the key compiled into this binary, requires the version to be newer
// than the running one, downloads into a directory only SYSTEM/root can write,
// checks size and SHA-256, and only then hands the package to the OS installer.
type Updater struct {
	dir        string
	emit       func(ipc.Event)
	principals func() []ipc.Principal
	http       *http.Client

	// Replaceable in tests; production uses the platform implementations.
	verify  func(*update.Manifest) error
	prepare func(dir string) error
	run         func(pkg, version, dir string, principals []ipc.Principal, onFail func(error)) error
	activeUsers func() []string
	openApp     func(userIDs []string)
	pause       time.Duration

	mu      sync.Mutex
	state   ipc.UpdateState
	busy    bool
	source  string              // management server that publishes updates (last one the app used)
	snoozes map[string]snooze   // user ID -> "Later" choice
	opened  map[string]string   // user ID -> version the app was opened for (once per version per boot)
}

// snooze is a user's "Later" for one version.
type snooze struct {
	Version string    `json:"version"`
	Until   time.Time `json:"until"`
}

// SnoozeDuration is how long "Later" hides a (non-mandatory) update.
const SnoozeDuration = 24 * time.Hour

// CheckInterval is how often the service looks for updates on its own.
const CheckInterval = 10 * time.Minute

// NewUpdater stores downloads under dir (created with system-only access).
func NewUpdater(dir string, emit func(ipc.Event), principals func() []ipc.Principal) *Updater {
	u := &Updater{
		dir:        dir,
		emit:       emit,
		principals: principals,
		http: &http.Client{
			Timeout: 15 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many redirects")
				}
				if !allowedScheme(req.URL) {
					return fmt.Errorf("refusing redirect to %s", req.URL.Scheme)
				}
				return nil
			},
		},
	}
	u.verify = func(m *update.Manifest) error { return m.Verify() }
	u.prepare = prepareUpdateDir
	u.run = runInstaller
	u.activeUsers = activeUserIDs
	u.openApp = relaunchApp
	u.pause = 3 * time.Second
	u.state = ipc.UpdateState{State: "idle", Supported: updatesSupported, CurrentVersion: update.Version}
	u.snoozes = map[string]snooze{}
	u.opened = map[string]string{}
	u.loadPrefs()
	return u
}

// UpdateStatus implements ipc.UpdateHandler.
func (u *Updater) UpdateStatus() ipc.UpdateState {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// CheckUpdate implements ipc.UpdateHandler.
func (u *Updater) CheckUpdate(_ ipc.Principal, serverURL string) (*ipc.UpdateState, error) {
	if !updatesSupported {
		st := u.UpdateStatus()
		return &st, nil
	}
	u.mu.Lock()
	if u.busy {
		st := u.state
		u.mu.Unlock()
		return &st, nil
	}
	u.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	m, err := u.fetchManifest(ctx, serverURL)
	if err == nil || errors.Is(err, errNotNewer) || errors.Is(err, errNoUpdatePublished) {
		u.rememberSource(serverURL)
	}
	switch {
	case errors.Is(err, errNoUpdatePublished):
		return u.set(ipc.UpdateState{State: "up_to_date"}), nil
	case errors.Is(err, errNotNewer):
		return u.set(ipc.UpdateState{State: "up_to_date", LatestVersion: m.Version}), nil
	case err != nil:
		return nil, err
	}
	return u.set(ipc.UpdateState{State: "available", LatestVersion: m.Version, Mandatory: m.Mandatory, Notes: m.Notes}), nil
}

// InstallUpdate implements ipc.UpdateHandler. It returns once the download
// has started; progress is broadcast as update.state events.
func (u *Updater) InstallUpdate(p ipc.Principal, serverURL string) error {
	if !updatesSupported {
		return errors.New("automatic updates are not supported on this platform")
	}
	u.mu.Lock()
	if u.busy {
		u.mu.Unlock()
		return errors.New("an update is already in progress")
	}
	u.busy = true
	u.mu.Unlock()

	log.Printf("update: install requested by %s", p.Username)
	go func() {
		err := u.install(serverURL, p)
		u.mu.Lock()
		u.busy = false
		u.mu.Unlock()
		if err != nil {
			log.Printf("update: %v", err)
			u.set(ipc.UpdateState{State: "failed", Error: err.Error(), LatestVersion: u.UpdateStatus().LatestVersion})
		}
	}()
	return nil
}

func (u *Updater) install(serverURL string, requester ipc.Principal) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	m, err := u.fetchManifest(ctx, serverURL)
	if errors.Is(err, errNotNewer) {
		return fmt.Errorf("version %s is not newer than the installed %s", m.Version, update.Version)
	}
	if err != nil {
		return err
	}
	base := ipc.UpdateState{LatestVersion: m.Version, Mandatory: m.Mandatory, Notes: m.Notes}

	if err := u.prepare(u.dir); err != nil {
		return fmt.Errorf("prepare update folder: %w", err)
	}
	cleanUpdateDir(u.dir)

	src, err := resolvePackageURL(serverURL, m.URL)
	if err != nil {
		return err
	}
	final := filepath.Join(u.dir, m.FileName)
	if err := u.download(ctx, src, final, m, base); err != nil {
		return err
	}

	// Record whose app to reopen now: announcing "installing" makes the apps
	// quit, after which they are no longer connected.
	reopen := u.appUsers(requester)

	st := base
	st.State = "installing"
	u.set(st)
	log.Printf("update: installing %s %s", m.Platform, m.Version)

	// Give the GUIs a moment to quit before the installer replaces their files.
	time.Sleep(u.pause)
	return u.run(final, m.Version, u.dir, reopen, func(err error) {
		// Called only if the installer failed while this daemon was still running.
		log.Printf("update: installer failed: %v", err)
		u.set(ipc.UpdateState{State: "failed", LatestVersion: m.Version, Error: "The installer failed. See the update log on this computer."})
		// The app quit for the install; bring it back.
		if info, ok := readRelaunchMarker(u.dir); ok {
			go u.openApp(info.UserIDs)
		}
	})
}

var (
	errNoUpdatePublished = errors.New("no client update is published")
	errNotNewer          = errors.New("the published client is not newer than this one")
)

func (u *Updater) fetchManifest(ctx context.Context, serverURL string) (*update.Manifest, error) {
	base, err := parseServerURL(serverURL)
	if err != nil {
		return nil, err
	}
	endpoint := base.JoinPath("api", "v1", "client-updates", update.CurrentPlatform(), "latest")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ProIdentity-Access")
	res, err := u.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errNoUpdatePublished
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("check for updates: server returned HTTP %d", res.StatusCode)
	}
	var m update.Manifest
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&m); err != nil {
		return nil, fmt.Errorf("read update manifest: %w", err)
	}
	// Nothing older or equal is ever installed, so it needn't be verified
	// (servers may still publish an unsigned manifest from before 0.7.2).
	if !update.Newer(m.Version, update.Version) {
		return &m, errNotNewer
	}
	if err := u.verify(&m); err != nil {
		return nil, fmt.Errorf("update rejected: %w", err)
	}
	if m.Platform != update.CurrentPlatform() {
		return nil, fmt.Errorf("update rejected: built for %s, not %s", m.Platform, update.CurrentPlatform())
	}
	return &m, nil
}

func (u *Updater) download(ctx context.Context, src *url.URL, final string, m *update.Manifest, base ipc.UpdateState) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ProIdentity-Access")
	res, err := u.http.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: server returned HTTP %d", res.StatusCode)
	}

	part := final + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	h := sha256.New()
	var written int64
	lastPct := -1
	buf := make([]byte, 256<<10)
	body := io.LimitReader(res.Body, m.Size+1)
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(part)
				return werr
			}
			h.Write(buf[:n])
			written += int64(n)
			if pct := int(written * 100 / m.Size); pct != lastPct && pct <= 100 {
				lastPct = pct
				st := base
				st.State = "downloading"
				st.Progress = pct
				u.set(st)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(part)
			return fmt.Errorf("download update: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(part)
		return err
	}
	if written != m.Size {
		os.Remove(part)
		return fmt.Errorf("update rejected: downloaded %d bytes, expected %d", written, m.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != strings.ToLower(m.SHA256) {
		os.Remove(part)
		return errors.New("update rejected: checksum does not match the signed manifest")
	}
	return os.Rename(part, final)
}

func (u *Updater) set(st ipc.UpdateState) *ipc.UpdateState {
	st.Supported = updatesSupported
	st.CurrentVersion = update.Version
	u.mu.Lock()
	u.state = st
	u.mu.Unlock()
	if u.emit != nil {
		if data, err := json.Marshal(st); err == nil {
			u.emit(ipc.Event{Type: ipc.EventUpdateState, Payload: data})
		}
	}
	return &st
}

func parseServerURL(raw string) (*url.URL, error) {
	s := strings.TrimRight(strings.TrimSpace(raw), "/")
	if s == "" {
		return nil, errors.New("no management server is configured")
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return nil, errors.New("invalid server URL")
	}
	if !allowedScheme(u) {
		return nil, errors.New("server URL must use https")
	}
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

// resolvePackageURL accepts a server-relative path or an absolute https URL.
func resolvePackageURL(serverURL, ref string) (*url.URL, error) {
	base, err := parseServerURL(serverURL)
	if err != nil {
		return nil, err
	}
	r, err := url.Parse(ref)
	if err != nil {
		return nil, errors.New("invalid package URL")
	}
	if r.IsAbs() {
		if !allowedScheme(r) {
			return nil, errors.New("package URL must use https")
		}
		return r, nil
	}
	if !strings.HasPrefix(r.Path, "/") {
		return nil, errors.New("invalid package URL")
	}
	return base.ResolveReference(r), nil
}

// allowedScheme permits https, and plain http only to this machine (development).
func allowedScheme(u *url.URL) bool {
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
	return false
}

// cleanUpdateDir removes earlier downloads, keeping the relaunch marker.
func cleanUpdateDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() == relaunchMarker {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".msi") || strings.HasSuffix(name, ".pkg") ||
			strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".plist") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// relaunchMarker records which users had the app open when an update started,
// so the new daemon can bring the app back after the upgrade.
const relaunchMarker = "relaunch.json"

type relaunchInfo struct {
	Version  string   `json:"version"`
	UserIDs  []string `json:"user_ids"`
	Started  string   `json:"started"`
}

func writeRelaunchMarker(dir, version string, principals []ipc.Principal) {
	info := relaunchInfo{Version: version, Started: time.Now().UTC().Format(time.RFC3339)}
	for _, p := range principals {
		info.UserIDs = append(info.UserIDs, p.UserID)
	}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(filepath.Join(dir, relaunchMarker), data, 0o600); err != nil {
		log.Printf("update: write relaunch marker: %v", err)
	}
}

func readRelaunchMarker(dir string) (*relaunchInfo, bool) {
	path := filepath.Join(dir, relaunchMarker)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	os.Remove(path)
	var info relaunchInfo
	if json.Unmarshal(data, &info) != nil {
		return nil, false
	}
	return &info, true
}

// ResumeAfterUpdate runs at daemon start: reports how the last update went
// and relaunches the app for users who had it open.
func (u *Updater) ResumeAfterUpdate() {
	info, ok := readRelaunchMarker(u.dir)
	if !ok {
		return
	}
	if info.Version == update.Version {
		log.Printf("update: now running %s", update.Version)
	} else {
		log.Printf("update: %s was not installed (still %s)", info.Version, update.Version)
		u.set(ipc.UpdateState{State: "failed", LatestVersion: info.Version,
			Error: "The last update did not complete. The previous version is still installed."})
	}
	users := info.UserIDs
	if len(users) == 0 {
		// Written by a version that captured the list too late (<= 0.7.3):
		// reopen the app for whoever is signed in.
		users = u.activeUsers()
	}
	go u.openApp(users)
}
