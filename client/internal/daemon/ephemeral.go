package daemon

import (
	"log"
	"time"

	"wg-client/internal/ipc"
)

// One-time tunnels: managed VPN sessions are imported with
// ImportEphemeralTunnel and live only for their server session. The app
// deletes them when the session ends; this loop is the safety net for when
// the app can't (it was killed, crashed or the user was logged off), so a
// dead session's keys don't linger in the service.

const (
	ephemeralReapInterval = 30 * time.Second
	// A new tunnel is connected right after import; give it this long.
	ephemeralStartGrace = 2 * time.Minute
	// WireGuard re-handshakes every 2 minutes while the peer exists (25 s
	// keepalive); no handshake for this long means the server removed the
	// peer, i.e. the session is over.
	ephemeralHandshakeTimeout = 5 * time.Minute
	// Connected but never completed a handshake.
	ephemeralFirstHandshakeTimeout = 3 * time.Minute
)

// ephemeralExpired reports whether a one-time tunnel should be removed.
func ephemeralExpired(status ipc.TunnelStatus, created, upSince time.Time, lastHandshake int64, now time.Time) bool {
	switch status {
	case ipc.StatusConnected:
		if lastHandshake > 0 {
			return now.Sub(time.Unix(lastHandshake, 0)) > ephemeralHandshakeTimeout
		}
		return !upSince.IsZero() && now.Sub(upSince) > ephemeralFirstHandshakeTimeout
	case ipc.StatusConnecting:
		return now.Sub(created) > ephemeralStartGrace+ephemeralFirstHandshakeTimeout
	default: // disconnected, error: the session isn't coming back
		return now.Sub(created) > ephemeralStartGrace
	}
}

func (m *TunnelManager) reapEphemeralLoop() {
	ticker := time.NewTicker(ephemeralReapInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.reapEphemeral(time.Now())
	}
}

func (m *TunnelManager) reapEphemeral(now time.Time) {
	m.mu.RLock()
	var candidates []*Tunnel
	for _, t := range m.tunnels {
		if t.Ephemeral {
			candidates = append(candidates, t)
		}
	}
	m.mu.RUnlock()

	for _, t := range candidates {
		status := t.Status()
		var lastHandshake int64
		if status == ipc.StatusConnected {
			if st, err := t.Stats(); err == nil {
				lastHandshake = st.LastHandshake
			}
		}
		t.mu.RLock()
		created, upSince := t.created, t.upSince
		t.mu.RUnlock()
		if !ephemeralExpired(status, created, upSince, lastHandshake, now) {
			continue
		}

		m.mu.Lock()
		if m.tunnels[t.Config.ID] != t {
			m.mu.Unlock()
			continue // already removed
		}
		delete(m.tunnels, t.Config.ID)
		m.mu.Unlock()

		log.Printf("one-time tunnel %q (%s) removed: session over", t.Config.Name, t.Config.ID)
		_ = t.Stop()
		info := t.Info()
		info.Status = ipc.StatusDisconnected
		t.forgetKeys()
		m.emitChanged(info)
	}
}
