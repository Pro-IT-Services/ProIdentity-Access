package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"wg-client/internal/ipc"
	"wg-client/internal/managed"
)

// WireGuard profiles assigned by the admin ("wgp:<id>" in the frontend).
//
// A profile is one WireGuard peer, so the server lets one device at a time
// have it: the app claims it (and gets the config), renews the claim while
// connected and releases it on disconnect. The config is imported as a
// one-time tunnel, never written to disk.

const (
	wgpPrefix = "wgp:"
	// Well inside the server's 3-minute claim lifetime.
	wgpHeartbeatEvery = 45 * time.Second
)

type wgpSession struct {
	tunnelID string
	name     string
	stop     context.CancelFunc
}

// WGProfileView is an assigned WireGuard profile as the frontend shows it.
type WGProfileView struct {
	ID          string                   `json:"id"` // "wgp:<server id>", the tunnel ID while connected
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Shared      bool                     `json:"shared"`
	Address     string                   `json:"address,omitempty"`
	Endpoint    string                   `json:"endpoint,omitempty"`
	InUseBy     *managed.WGProfileHolder `json:"in_use_by,omitempty"`
}

// ManagedListWireGuardProfiles returns the WireGuard profiles assigned to the
// user, with who has each one up.
func (a *App) ManagedListWireGuardProfiles() ([]WGProfileView, error) {
	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()
	views := []WGProfileView{}
	if mc == nil {
		return views, nil
	}
	profiles, err := mc.ListWireGuardProfiles()
	if err != nil {
		if handled := a.handleManagedAuthError(err); handled != nil {
			return nil, handled
		}
		return nil, err
	}
	for _, p := range profiles {
		views = append(views, WGProfileView{
			ID: wgpPrefix + p.ID, Name: p.Name, Description: p.Description, Shared: p.Shared,
			Address: p.Address, Endpoint: p.Endpoint, InUseBy: p.InUseBy,
		})
	}
	return views, nil
}

// wgpInUseText is the reason a profile can't be connected right now.
func wgpInUseText(h managed.WGProfileHolder) string {
	if h.You && h.Device != "" {
		return strings.ReplaceAll(tr("err.wgProfileInUseYou"), "{device}", h.Device)
	}
	device := h.Device
	if device == "" {
		device = "?"
	}
	return strings.NewReplacer("{name}", h.Name, "{device}", device).Replace(tr("err.wgProfileInUse"))
}

// connectWGProfile claims the profile on the server and brings it up.
func (a *App) connectWGProfile(key string) error {
	id := strings.TrimPrefix(key, wgpPrefix)
	a.mMu.Lock()
	mc := a.mClient
	a.mMu.Unlock()
	if mc == nil {
		return fmt.Errorf("not logged in")
	}
	a.teardownWGProfile(id, false) // a stale local tunnel; the claim below renews

	name, config, err := mc.ClaimWireGuardProfile(id)
	if err != nil {
		var inUse *managed.WGProfileInUseError
		if errors.As(err, &inUse) {
			return errors.New(wgpInUseText(inUse.Holder))
		}
		if handled := a.handleManagedAuthError(err); handled != nil {
			return handled
		}
		return err
	}

	tunnel, err := a.client.ImportEphemeralTunnel(name, withPersistentKeepalive(config))
	if err != nil {
		go mc.ReleaseWireGuardProfile(id)
		return fmt.Errorf("import tunnel: %w", err)
	}
	if err := a.client.ConnectTunnel(tunnel.ID); err != nil {
		a.client.DeleteTunnel(tunnel.ID)
		go mc.ReleaseWireGuardProfile(id)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.mMu.Lock()
	a.mWGP[id] = &wgpSession{tunnelID: tunnel.ID, name: name, stop: cancel}
	a.mMu.Unlock()
	go a.wgProfileHeartbeat(ctx, mc, id, tunnel.ID)
	log.Printf("connected assigned WireGuard profile %q", name)
	return nil
}

// disconnectWGProfile takes the profile down and frees it on the server.
func (a *App) disconnectWGProfile(key string) error {
	a.teardownWGProfile(strings.TrimPrefix(key, wgpPrefix), true)
	return nil
}

// teardownWGProfile removes the local tunnel; release frees the server claim.
func (a *App) teardownWGProfile(id string, release bool) {
	a.mMu.Lock()
	s := a.mWGP[id]
	delete(a.mWGP, id)
	mc := a.mClient
	a.mMu.Unlock()
	if s == nil {
		return
	}
	if s.stop != nil {
		s.stop()
	}
	a.client.DisconnectTunnel(s.tunnelID)
	a.client.DeleteTunnel(s.tunnelID)
	if a.ctx != nil {
		// The service forgot the tunnel; tell the UI under the profile's ID.
		runtime.EventsEmit(a.ctx, ipc.EventTunnelChanged, ipc.TunnelInfo{
			ID: wgpPrefix + id, Name: s.name, Status: ipc.StatusDisconnected,
			Addresses: []string{}, DNS: []string{}, Peers: []ipc.PeerInfo{},
		})
	}
	if release && mc != nil {
		go releaseWGProfile(mc, id)
	}
}

// releaseWGProfile frees a claim, retrying briefly; if the server stays
// unreachable the claim lapses on its own.
func releaseWGProfile(mc *managed.Client, id string) {
	for i := 0; i < 3; i++ {
		if err := mc.ReleaseWireGuardProfile(id); err == nil ||
			errors.Is(err, managed.ErrAuthInvalid) || errors.Is(err, managed.ErrDeviceRevoked) {
			return
		}
		time.Sleep(3 * time.Second)
	}
}

// releaseAllWGProfiles takes every profile down (logout, quit). With wait > 0
// it waits up to that long for the server to free the claims.
func (a *App) releaseAllWGProfiles(wait time.Duration) {
	a.mMu.Lock()
	ids := make([]string, 0, len(a.mWGP))
	for id := range a.mWGP {
		ids = append(ids, id)
	}
	mc := a.mClient
	a.mMu.Unlock()
	for _, id := range ids {
		a.teardownWGProfile(id, false)
	}
	if mc == nil || len(ids) == 0 {
		return
	}
	done := make(chan struct{})
	go func() {
		for _, id := range ids {
			_ = mc.ReleaseWireGuardProfile(id)
		}
		close(done)
	}()
	if wait > 0 {
		select {
		case <-done:
		case <-time.After(wait):
		}
	}
}

// wgProfileHeartbeat renews the claim while the tunnel is up. It ends the
// connection when the server says the profile is no longer this device's
// (taken after the claim lapsed, unassigned, deleted, logged out), and frees
// the claim when the tunnel went down some other way.
func (a *App) wgProfileHeartbeat(ctx context.Context, mc *managed.Client, id, tunnelID string) {
	ticker := time.NewTicker(wgpHeartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if !a.daemonTunnelUp(tunnelID) {
			log.Printf("assigned WireGuard profile %s went down; releasing", id)
			a.teardownWGProfile(id, true)
			return
		}
		err := mc.HeartbeatWireGuardProfile(id)
		if err == nil {
			continue
		}
		var inUse *managed.WGProfileInUseError
		var apiErr *managed.APIError
		switch {
		case errors.As(err, &inUse):
			a.lostWGProfile(id, wgpInUseText(inUse.Holder))
			return
		case errors.As(err, &apiErr) && apiErr.Status == 404:
			a.lostWGProfile(id, tr("err.wgProfileRemoved"))
			return
		case errors.As(err, &apiErr) && apiErr.Status == 409:
			a.lostWGProfile(id, tr("err.wgProfileReleased"))
			return
		case errors.Is(err, managed.ErrAuthInvalid) || errors.Is(err, managed.ErrDeviceRevoked):
			// The claim can't be renewed and would lapse while still up.
			a.teardownWGProfile(id, false)
			_ = a.handleManagedAuthError(err)
			return
		default:
			log.Printf("heartbeat for assigned WireGuard profile %s failed: %v (keeping it up)", id, err)
		}
	}
}

func (a *App) lostWGProfile(id, reason string) {
	a.mMu.Lock()
	name := ""
	if s := a.mWGP[id]; s != nil {
		name = s.name
	}
	a.mMu.Unlock()
	log.Printf("assigned WireGuard profile %q disconnected: %s", name, reason)
	a.teardownWGProfile(id, false)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "wgprofile.lost", map[string]string{"name": name, "reason": reason})
	}
}

func (a *App) daemonTunnelUp(tunnelID string) bool {
	tunnels, err := a.client.ListTunnels()
	if err != nil {
		return true // can't tell; don't drop a working connection
	}
	for _, t := range tunnels {
		if t.ID == tunnelID {
			return t.Status == ipc.StatusConnected || t.Status == ipc.StatusConnecting
		}
	}
	return false
}

// wgpKeyByTunnelID returns "wgp:<id>" for a daemon tunnel, or "".
func (a *App) wgpKeyByTunnelID(daemonTunnelID string) string {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	for id, s := range a.mWGP {
		if s.tunnelID == daemonTunnelID {
			return wgpPrefix + id
		}
	}
	return ""
}

// wgpTunnelID returns the daemon tunnel for "wgp:<id>", or "".
func (a *App) wgpTunnelID(key string) string {
	a.mMu.Lock()
	defer a.mMu.Unlock()
	if s := a.mWGP[strings.TrimPrefix(key, wgpPrefix)]; s != nil {
		return s.tunnelID
	}
	return ""
}

// withPersistentKeepalive adds a 25 s keepalive to each [Peer] that has
// none. The service drops a one-time tunnel that goes minutes without a
// handshake, which an idle connection without keepalive would.
func withPersistentKeepalive(config string) string {
	lines := strings.Split(config, "\n")
	out := make([]string, 0, len(lines)+2)
	inPeer, hasKA := false, false
	flush := func() {
		if inPeer && !hasKA {
			// Insert before trailing blank lines of the section.
			i := len(out)
			for i > 0 && strings.TrimSpace(out[i-1]) == "" {
				i--
			}
			out = append(out[:i], append([]string{"PersistentKeepalive = 25"}, out[i:]...)...)
		}
	}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			flush()
			inPeer = strings.EqualFold(strings.Trim(t, "[] "), "peer")
			hasKA = false
		} else if inPeer {
			if k, _, ok := strings.Cut(t, "="); ok && strings.EqualFold(strings.TrimSpace(k), "persistentkeepalive") {
				hasKA = true
			}
		}
		out = append(out, line)
	}
	flush()
	return strings.Join(out, "\n")
}
