//go:build windows

package main

import (
	"context"
	_ "embed"
	"runtime"
	"sync"

	"fyne.io/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"wg-client/internal/ipc"
)

// embed the ICO — LoadImageW on Windows requires ICO format, not PNG.
//
//go:embed build/windows/icon.ico
var trayIconICO []byte

// ── Globals ───────────────────────────────────────────────────────────────────

var (
	trayCtx context.Context
	trayApp *App

	trayRefreshCh = make(chan struct{}, 1)

	trayStartOnce sync.Once
	trayLoopOnce  sync.Once

	trayMenuMu     sync.Mutex
	trayMenuCancel context.CancelFunc

	animMu   sync.Mutex
	animStop chan struct{}
)

// ── Connection state ──────────────────────────────────────────────────────────

func connectionState() string {
	if trayApp == nil || !trayApp.client.IsConnected() {
		return "disconnected"
	}
	_, connected := trayApp.trayManagedClient()
	if len(connected) > 0 {
		return "connected"
	}
	tunnels, _ := trayApp.client.ListTunnels()
	for _, t := range tunnels {
		switch t.Status {
		case ipc.StatusConnected:
			return "connected"
		case ipc.StatusConnecting:
			return "connecting"
		}
	}
	return "disconnected"
}

func setConnectionTooltip(state string) {
	switch state {
	case "connected":
		systray.SetTooltip("ProIdentity VPN — Connected")
	case "connecting":
		systray.SetTooltip("ProIdentity VPN — Connecting…")
	default:
		systray.SetTooltip("ProIdentity VPN")
	}
}

// ── Tray lifecycle ────────────────────────────────────────────────────────────

func signalTrayRefresh() {
	select {
	case trayRefreshCh <- struct{}{}:
	default:
	}
}

func onTrayReady() {
	systray.SetIcon(trayIconICO)
	systray.SetTooltip("ProIdentity VPN")
	systray.SetOnTapped(showTrayPopover)
	buildTrayMenu()
	trayLoopOnce.Do(func() {
		go func() {
			for range trayRefreshCh {
				buildTrayMenu()
			}
		}()
	})
}

// ── Menu building ─────────────────────────────────────────────────────────────

func buildTrayMenu() {
	trayMenuMu.Lock()
	defer trayMenuMu.Unlock()

	if trayMenuCancel != nil {
		trayMenuCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	trayMenuCancel = cancel

	state := connectionState()
	setConnectionTooltip(state)
	systray.ResetMenu()
	addBottomItems(ctx)
	return

	hasItems := false

	if trayApp == nil {
		addPlaceholder()
		addBottomItems(ctx)
		return
	}

	// ── Managed servers ───────────────────────────────────────────────────
	mc, connected := trayApp.trayManagedClient()
	if mc != nil {
		servers, err := mc.ListServers()
		if err == nil {
			seenServers := make(map[string]struct{}, len(servers))
			for _, srv := range servers {
				srv := srv
				key := srv.ID
				if key == "" {
					key = srv.Name
				}
				if _, ok := seenServers[key]; ok {
					continue
				}
				seenServers[key] = struct{}{}

				isConnected := connected[srv.ID]
				item := systray.AddMenuItem(srv.Name, "")
				if isConnected {
					item.Check()
				}
				go func() {
					for {
						select {
						case <-ctx.Done():
							return
						case <-item.ClickedCh:
							if isConnected {
								trayApp.ManagedDisconnectServer(srv.ID)
								signalTrayRefresh()
							} else if trayCtx != nil {
								wailsruntime.WindowShow(trayCtx)
								wailsruntime.WindowSetAlwaysOnTop(trayCtx, true)
								wailsruntime.WindowSetAlwaysOnTop(trayCtx, false)
								wailsruntime.EventsEmit(trayCtx, "tray:connect-server", srv.ID)
							}
						}
					}
				}()
				hasItems = true
			}
		}
	}

	// ── Standalone daemon tunnels ─────────────────────────────────────────
	if trayApp.client.IsConnected() {
		tunnels, _ := trayApp.client.ListTunnels()
		hiddenDaemonTunnels := trayApp.trayDaemonTunnelIDsHiddenFromStandalone()
		seenTunnels := make(map[string]struct{}, len(tunnels))
		for _, t := range tunnels {
			t := t
			if t.IsManaged || hiddenDaemonTunnels[t.ID] {
				continue
			}
			key := t.ID
			if key == "" {
				key = t.Name
			}
			if _, ok := seenTunnels[key]; ok {
				continue
			}
			seenTunnels[key] = struct{}{}

			item := systray.AddMenuItem(tunnelLabel(t), "")
			switch t.Status {
			case ipc.StatusConnected:
				item.Check()
			case ipc.StatusConnecting, ipc.StatusError:
				item.Disable()
			}
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-item.ClickedCh:
						if t.Status == ipc.StatusConnected {
							trayApp.client.DisconnectTunnel(t.ID)
						} else {
							trayApp.client.ConnectTunnel(t.ID)
						}
					}
				}
			}()
			hasItems = true
		}
	}

	if !hasItems {
		addPlaceholder()
	}
	addBottomItems(ctx)
}

func addPlaceholder() {
	item := systray.AddMenuItem("No tunnels", "")
	item.Disable()
}

func addBottomItems(ctx context.Context) {
	mShow := systray.AddMenuItem("Show", "Show window")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-mShow.ClickedCh:
				showTrayPopover()
			}
		}
	}()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit ProIdentity")
	go func() {
		select {
		case <-ctx.Done():
		case <-mQuit.ClickedCh:
			systray.Quit()
			if trayCtx != nil {
				wailsruntime.Quit(trayCtx)
			}
		}
	}()
}

func showTrayPopover() {
	if trayCtx == nil {
		return
	}
	const width = 430
	const height = 580
	wailsruntime.WindowSetMinSize(trayCtx, width, height)
	wailsruntime.WindowSetMaxSize(trayCtx, width, height)
	wailsruntime.WindowSetSize(trayCtx, width, height)
	if screens, err := wailsruntime.ScreenGetAll(trayCtx); err == nil && len(screens) > 0 {
		screen := screens[0]
		for _, s := range screens {
			if s.IsCurrent || s.IsPrimary {
				screen = s
				break
			}
		}
		x := screen.Size.Width - width - 24
		y := screen.Size.Height - height - 88
		if x < 16 {
			x = 16
		}
		if y < 16 {
			y = 16
		}
		wailsruntime.WindowSetPosition(trayCtx, x, y)
	}
	wailsruntime.WindowSetAlwaysOnTop(trayCtx, true)
	wailsruntime.WindowShow(trayCtx)
	wailsruntime.WindowUnminimise(trayCtx)
	wailsruntime.Show(trayCtx)
	wailsruntime.EventsEmit(trayCtx, "tray:popover")
}

func showMainWindow() {
	if trayCtx == nil {
		return
	}
	wailsruntime.WindowSetMaxSize(trayCtx, 0, 0)
	wailsruntime.WindowSetMinSize(trayCtx, 800, 580)
	wailsruntime.WindowSetSize(trayCtx, 960, 660)
	wailsruntime.WindowCenter(trayCtx)
	wailsruntime.WindowShow(trayCtx)
	wailsruntime.WindowUnminimise(trayCtx)
	wailsruntime.Show(trayCtx)
	wailsruntime.WindowSetAlwaysOnTop(trayCtx, true)
	wailsruntime.WindowSetAlwaysOnTop(trayCtx, false)
	wailsruntime.EventsEmit(trayCtx, "tray:show-main")
}

func tunnelLabel(t ipc.TunnelInfo) string {
	switch t.Status {
	case ipc.StatusConnecting:
		return t.Name + "…"
	case ipc.StatusError:
		return t.Name + " ⚠"
	default:
		return t.Name
	}
}

// ── App integration ───────────────────────────────────────────────────────────

// setupTray starts the systray on its own OS-locked goroutine so that the
// hidden window and the GetMessage loop live on the same OS thread —
// required for Windows message dispatch to work correctly.
func (a *App) setupTray(ctx context.Context) {
	trayCtx = ctx
	trayApp = a
	trayStartOnce.Do(func() {
		go func() {
			runtime.LockOSThread()
			systray.Run(onTrayReady, nil)
		}()
	})
	signalTrayRefresh()
}

func (a *App) teardownTray() {
	animMu.Lock()
	if animStop != nil {
		close(animStop)
		animStop = nil
	}
	animMu.Unlock()
	systray.Quit()
}
