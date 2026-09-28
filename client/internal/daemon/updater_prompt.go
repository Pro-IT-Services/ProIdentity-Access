package daemon

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"wg-client/internal/ipc"
)

// The service checks for updates on its own every CheckInterval, using the
// management server the app last used, and makes sure every signed-in user
// sees the prompt: running apps get the update.state event and bring their
// window forward; for users without the app open, the service starts it in
// their session (once per version per boot). "Later" is remembered per user.

const prefsFile = "update-prefs.json"

type updatePrefs struct {
	Source  string            `json:"source"`
	Snoozes map[string]snooze `json:"snoozes"`
}

// Run checks for updates periodically until stop is closed.
func (u *Updater) Run(stop <-chan struct{}) {
	if !updatesSupported {
		return
	}
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			u.periodicCheck()
			timer.Reset(CheckInterval)
		}
	}
}

func (u *Updater) periodicCheck() {
	u.mu.Lock()
	source, busy := u.source, u.busy
	u.mu.Unlock()
	if source == "" || busy {
		return
	}
	st, err := u.CheckUpdate(ipc.Principal{}, source)
	if err != nil {
		log.Printf("update: periodic check: %v", err)
		return
	}
	if st.State == "available" {
		u.promptUsers(st)
	}
}

// promptUsers opens the app for signed-in users who don't have it running and
// haven't chosen "Later" for this version. Running apps already got the event.
func (u *Updater) promptUsers(st *ipc.UpdateState) {
	connected := map[string]bool{}
	for _, p := range u.principals() {
		connected[p.UserID] = true
	}
	for _, user := range u.activeUsers() {
		if connected[user] || (!st.Mandatory && u.snoozed(user, st.LatestVersion)) {
			continue
		}
		u.mu.Lock()
		already := u.opened[user] == st.LatestVersion
		u.opened[user] = st.LatestVersion
		u.mu.Unlock()
		if already {
			continue
		}
		log.Printf("update: opening the app for %s to offer %s", user, st.LatestVersion)
		go u.openApp([]string{user})
	}
}

// SnoozeUpdate implements ipc.UpdateHandler ("Later").
func (u *Updater) SnoozeUpdate(p ipc.Principal, version string) error {
	if !p.Valid() || version == "" {
		return nil
	}
	u.mu.Lock()
	u.snoozes[p.UserID] = snooze{Version: version, Until: time.Now().Add(SnoozeDuration)}
	u.mu.Unlock()
	u.savePrefs()
	return nil
}

func (u *Updater) snoozed(userID, version string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	s, ok := u.snoozes[userID]
	return ok && s.Version == version && time.Now().Before(s.Until)
}

// appUsers lists users whose app should reopen after an install: everyone
// with the app connected, plus whoever asked for the update.
func (u *Updater) appUsers(requester ipc.Principal) []ipc.Principal {
	seen := map[string]bool{}
	var out []ipc.Principal
	add := func(p ipc.Principal) {
		if p.Valid() && !seen[p.UserID] {
			seen[p.UserID] = true
			out = append(out, p)
		}
	}
	add(requester)
	for _, p := range u.principals() {
		add(p)
	}
	return out
}

func (u *Updater) rememberSource(serverURL string) {
	parsed, err := parseServerURL(serverURL)
	if err != nil {
		return
	}
	src := parsed.String()
	u.mu.Lock()
	changed := u.source != src
	u.source = src
	u.mu.Unlock()
	if changed {
		u.savePrefs()
	}
}

func (u *Updater) loadPrefs() {
	data, err := os.ReadFile(filepath.Join(u.dir, prefsFile))
	if err != nil {
		return
	}
	var p updatePrefs
	if json.Unmarshal(data, &p) != nil {
		return
	}
	if _, err := parseServerURL(p.Source); err == nil {
		u.source = p.Source
	}
	for user, s := range p.Snoozes {
		if time.Now().Before(s.Until) {
			u.snoozes[user] = s
		}
	}
}

func (u *Updater) savePrefs() {
	u.mu.Lock()
	p := updatePrefs{Source: u.source, Snoozes: map[string]snooze{}}
	for k, v := range u.snoozes {
		if time.Now().Before(v.Until) {
			p.Snoozes[k] = v
		}
	}
	u.mu.Unlock()
	if err := u.prepare(u.dir); err != nil {
		log.Printf("update: save preferences: %v", err)
		return
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(filepath.Join(u.dir, prefsFile), data, 0o600); err != nil {
		log.Printf("update: save preferences: %v", err)
	}
}
