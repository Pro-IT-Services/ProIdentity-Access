//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

extern void trayStartCallback(void);
void dispatchTrayStart(void);
*/
import "C"
import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"wg-client/internal/ipc"
)

// ── Icon generation ───────────────────────────────────────────────────────────

// shieldPNG returns a 22×22 template-mode PNG for the given state:
//
//	"on"   – solid filled shield  (connected)
//	"off"  – outline-only shield  (disconnected)
//	"half" – shield with horizontal stripes (connecting / in-progress)
func shieldPNG(state string) []byte {
	const sz = 22
	img := image.NewNRGBA(image.Rect(0, 0, sz, sz))
	black := color.NRGBA{0, 0, 0, 255}

	// Returns true when pixel (x,y) lies inside the shield silhouette.
	inside := func(x, y int) bool {
		if x < 0 || x >= sz || y < 0 || y >= sz {
			return false
		}
		fx := float64(x) / float64(sz-1)
		fy := float64(y) / float64(sz-1)
		if fy < 0.05 {
			return false // top margin
		}
		if fy < 0.52 {
			return fx >= 0.09 && fx <= 0.91 // straight sides
		}
		// Tapers to a point at the bottom
		t := (fy - 0.52) / 0.48
		pad := 0.09 + t*(0.5-0.09)
		return fx >= pad && fx <= 1.0-pad
	}

	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			if !inside(x, y) {
				continue
			}
			var draw bool
			switch state {
			case "on":
				draw = true
			case "half":
				// Horizontal stripes: filled rows alternating every 2px
				draw = (y/2)%2 == 0
			case "off":
				// 2-pixel inset border
				draw = !inside(x-1, y) || !inside(x+1, y) ||
					!inside(x, y-1) || !inside(x, y+1) ||
					!inside(x-2, y) || !inside(x+2, y) ||
					!inside(x, y-2) || !inside(x, y+2)
			}
			if draw {
				img.Set(x, y, black)
			}
		}
	}

	// For the solid "on" icon, cut out a small lock body in negative space
	// so it's visually distinct from the "half" icon.
	if state == "on" {
		// Lock shackle: small arc at top-center
		cx, cy := float64(sz)/2, float64(sz)*0.38
		r := float64(sz) * 0.13
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				dx := float64(x) - cx
				dy := float64(y) - cy
				dist := math.Sqrt(dx*dx + dy*dy)
				// Hollow arc (only top half, 1-pixel ring)
				if dist >= r-0.7 && dist <= r+0.7 && dy <= 0 {
					img.Set(x, y, color.NRGBA{})
				}
			}
		}
		// Lock body: small rectangle below arc
		lx := int(cx) - 2
		rx := int(cx) + 2
		ty := int(cy) + 1
		by := ty + 4
		for y := ty; y <= by; y++ {
			for x := lx; x <= rx; x++ {
				if x >= 0 && x < sz && y >= 0 && y < sz {
					img.Set(x, y, color.NRGBA{})
				}
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Pre-generate the three state icons at init time.
var (
	iconOn   = shieldPNG("on")   // connected
	iconOff  = shieldPNG("off")  // disconnected
	iconHalf = shieldPNG("half") // connecting (used in animation)
)

// ── Connection state icon ─────────────────────────────────────────────────────

var (
	animMu   sync.Mutex
	animStop chan struct{}
)

// setConnectionIcon picks the right icon and, for "connecting", starts a
// blinking animation between iconOff and iconHalf.
func setConnectionIcon(state string) {
	animMu.Lock()
	if animStop != nil {
		close(animStop)
		animStop = nil
	}
	animMu.Unlock()

	switch state {
	case "connected":
		systray.SetTemplateIcon(iconOn, iconOn)
	case "connecting":
		systray.SetTemplateIcon(iconHalf, iconHalf)
		stop := make(chan struct{})
		animMu.Lock()
		animStop = stop
		animMu.Unlock()
		go func() {
			tick := time.NewTicker(500 * time.Millisecond)
			defer tick.Stop()
			frame := false
			for {
				select {
				case <-stop:
					return
				case <-tick.C:
					if frame {
						systray.SetTemplateIcon(iconHalf, iconHalf)
					} else {
						systray.SetTemplateIcon(iconOff, iconOff)
					}
					frame = !frame
				}
			}
		}()
	default:
		systray.SetTemplateIcon(iconOff, iconOff)
	}
}

// connectionState derives the overall icon state from managed sessions +
// daemon tunnels.
func connectionState() string {
	if trayApp == nil {
		return "disconnected"
	}
	_, connected := trayApp.trayManagedClient()
	if len(connected) > 0 {
		return "connected"
	}
	if trayApp.client.IsConnected() {
		tunnels, _ := trayApp.client.ListTunnels()
		for _, t := range tunnels {
			switch t.Status {
			case ipc.StatusConnected:
				return "connected"
			case ipc.StatusConnecting:
				return "connecting"
			}
		}
	}
	return "disconnected"
}

// ── Tray lifecycle ────────────────────────────────────────────────────────────

var (
	trayCtx     context.Context
	trayApp     *App
	trayStartFn func()
	trayEndFn   func()

	trayRefreshCh = make(chan struct{}, 1)

	trayStartOnce sync.Once
	trayLoopOnce  sync.Once

	trayMenuMu     sync.Mutex
	trayMenuCancel context.CancelFunc
)

func init() {
	trayStartFn, trayEndFn = systray.RunWithExternalLoop(onTrayReady, nil)
}

func signalTrayRefresh() {
	select {
	case trayRefreshCh <- struct{}{}:
	default:
	}
}

func onTrayReady() {
	setConnectionIcon("disconnected")
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

	// Update icon to reflect current overall state.
	setConnectionIcon(connectionState())

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
							} else {
								if trayCtx != nil {
									runtime.WindowShow(trayCtx)
									runtime.WindowSetAlwaysOnTop(trayCtx, true)
									runtime.WindowSetAlwaysOnTop(trayCtx, false)
									runtime.EventsEmit(trayCtx, "tray:connect-server", srv.ID)
								}
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
				runtime.Quit(trayCtx)
			}
		}
	}()
}

func showTrayPopover() {
	if trayCtx == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("tray popover dispatch failed: %v", r)
			}
		}()
		time.Sleep(25 * time.Millisecond)
		runtime.WindowShow(trayCtx)
		runtime.WindowUnminimise(trayCtx)
		runtime.Show(trayCtx)
		runtime.EventsEmit(trayCtx, "tray:popover")
	}()
}

func showMainWindow() {
	if trayCtx == nil {
		return
	}
	runtime.WindowSetMaxSize(trayCtx, 0, 0)
	runtime.WindowSetMinSize(trayCtx, 800, 580)
	runtime.WindowSetSize(trayCtx, 960, 660)
	runtime.WindowCenter(trayCtx)
	runtime.WindowShow(trayCtx)
	runtime.WindowUnminimise(trayCtx)
	runtime.Show(trayCtx)
	runtime.WindowSetAlwaysOnTop(trayCtx, true)
	runtime.WindowSetAlwaysOnTop(trayCtx, false)
	runtime.EventsEmit(trayCtx, "tray:show-main")
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

// ── CGo bridge ───────────────────────────────────────────────────────────────

//export trayStartCallback
func trayStartCallback() {
	if trayStartFn != nil {
		trayStartFn()
	}
}

func (a *App) setupTray(ctx context.Context) {
	trayCtx = ctx
	trayApp = a
	trayStartOnce.Do(func() {
		C.dispatchTrayStart()
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
	if trayEndFn != nil {
		trayEndFn()
	}
}
