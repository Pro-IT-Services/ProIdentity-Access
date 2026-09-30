package main

import (
	"errors"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wg-client/internal/ipc"
	"wg-client/internal/update"
)

// Updates are checked, downloaded, verified and installed by the daemon, which
// runs as LocalSystem / root. Users without admin rights can therefore update;
// the app only names the management server and shows the prompt.

func (a *App) updateServerURL() (string, error) {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	if a.mSettings == nil || strings.TrimSpace(a.mSettings.ServerURL) == "" {
		return "", errors.New(tr("err.noManagementServer"))
	}
	return strings.TrimRight(a.mSettings.ServerURL, "/"), nil
}

// AppVersion is the installed client version.
func (a *App) AppVersion() string { return update.Version }

// CheckForUpdate asks the service whether the management server publishes a
// newer, correctly signed client for this platform.
func (a *App) CheckForUpdate() (*ipc.UpdateState, error) {
	serverURL, err := a.updateServerURL()
	if err != nil {
		return nil, err
	}
	if !a.client.IsConnected() {
		return nil, errors.New(tr("err.serviceNotRunning"))
	}
	return a.client.CheckUpdate(serverURL)
}

// InstallUpdate asks the service to download and install the update. The app
// quits when installation starts and is reopened afterwards.
func (a *App) InstallUpdate() error {
	serverURL, err := a.updateServerURL()
	if err != nil {
		return err
	}
	if !a.client.IsConnected() {
		return errors.New(tr("err.serviceNotRunning"))
	}
	return a.client.InstallUpdate(serverURL)
}

// SnoozeUpdate records "Later" for version with the service, so it doesn't
// reopen the app to offer this version again for a while.
func (a *App) SnoozeUpdate(version string) error {
	if !a.client.IsConnected() {
		return nil
	}
	return a.client.SnoozeUpdate(version)
}

// UpdateState returns the service's current update state.
func (a *App) UpdateState() (*ipc.UpdateState, error) {
	if !a.client.IsConnected() {
		return &ipc.UpdateState{State: "idle", CurrentVersion: update.Version}, nil
	}
	return a.client.UpdateStatus()
}

// onUpdateState forwards update progress to the UI. When installation starts
// the app quits so the installer can replace it; the service reopens it.
func (a *App) onUpdateState(st ipc.UpdateState) {
	runtime.EventsEmit(a.ctx, ipc.EventUpdateState, st)
	if st.State == "installing" {
		a.quitForUpdate.Do(func() {
			go func() {
				time.Sleep(1500 * time.Millisecond)
				runtime.Quit(a.ctx)
			}()
		})
	}
}
