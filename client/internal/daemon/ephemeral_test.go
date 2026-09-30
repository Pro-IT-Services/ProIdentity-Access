package daemon

import (
	"testing"
	"time"

	"wg-client/internal/ipc"
)

func TestEphemeralExpired(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	hs := func(d time.Duration) int64 { return now.Add(-d).Unix() }

	cases := []struct {
		name          string
		status        ipc.TunnelStatus
		created, up   time.Time
		lastHandshake int64
		want          bool
	}{
		{"just imported, not connected yet", ipc.StatusDisconnected, ago(30 * time.Second), time.Time{}, 0, false},
		{"disconnected and left behind", ipc.StatusDisconnected, ago(10 * time.Minute), time.Time{}, 0, true},
		{"failed and left behind", ipc.StatusError, ago(3 * time.Minute), time.Time{}, 0, true},
		{"connecting normally", ipc.StatusConnecting, ago(1 * time.Minute), time.Time{}, 0, false},
		{"stuck connecting", ipc.StatusConnecting, ago(10 * time.Minute), time.Time{}, 0, true},
		{"healthy, recent handshake", ipc.StatusConnected, ago(2 * time.Hour), ago(2 * time.Hour), hs(90 * time.Second), false},
		{"server removed the peer", ipc.StatusConnected, ago(2 * time.Hour), ago(2 * time.Hour), hs(6 * time.Minute), true},
		{"up, waiting for first handshake", ipc.StatusConnected, ago(1 * time.Minute), ago(1 * time.Minute), 0, false},
		{"up, never handshaked", ipc.StatusConnected, ago(5 * time.Minute), ago(4 * time.Minute), 0, true},
	}
	for _, c := range cases {
		if got := ephemeralExpired(c.status, c.created, c.up, c.lastHandshake, now); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
