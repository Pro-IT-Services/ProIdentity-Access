package daemon

import (
	"testing"

	"wg-client/internal/ipc"
)

func TestMgmtEscape(t *testing.T) {
	cases := map[string]string{
		"secret":       `"secret"`,
		`a"b`:          `"a\"b"`,
		`back\slash`:   `"back\\slash"`,
		`pw"\combo`:    `"pw\"\\combo"`,
	}
	for in, want := range cases {
		if got := mgmtEscape(in); got != want {
			t.Errorf("mgmtEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParamsID(t *testing.T) {
	if got := paramsID(ipc.OpenVPNConnectParams{ID: "abc"}); got != "abc" {
		t.Errorf("paramsID with ID = %q, want abc", got)
	}
	if got := paramsID(ipc.OpenVPNConnectParams{Name: "Corp"}); got != "ovpn-Corp" {
		t.Errorf("paramsID fallback = %q, want ovpn-Corp", got)
	}
}

func TestHandleStateTransitions(t *testing.T) {
	m := NewOpenVPNManager(nil) // nil broadcast is fine
	s := &ovpnSession{id: "s1", status: ipc.StatusConnecting, devType: "tun"}

	m.handleState(s, "1700000000,ASSIGN_IP,,,,")
	if s.status != ipc.StatusConnecting {
		t.Fatalf("ASSIGN_IP: status=%q, want connecting", s.status)
	}

	m.handleState(s, "1700000000,CONNECTED,SUCCESS,10.9.0.5,203.0.113.1,,")
	if s.status != ipc.StatusConnected {
		t.Fatalf("CONNECTED: status=%q, want connected", s.status)
	}
	if s.ip != "10.9.0.5" {
		t.Fatalf("CONNECTED: ip=%q, want 10.9.0.5", s.ip)
	}

	m.handleState(s, "1700000000,EXITING,SIGTERM,,,")
	if s.status != ipc.StatusDisconnected {
		t.Fatalf("EXITING: status=%q, want disconnected", s.status)
	}
}

func TestHandleStateExitingKeepsError(t *testing.T) {
	m := NewOpenVPNManager(nil)
	s := &ovpnSession{id: "s2", status: ipc.StatusConnecting}
	s.setError("authentication failed")
	m.handleState(s, "1700000000,EXITING,auth-failure,,,")
	if s.status != ipc.StatusError {
		t.Fatalf("EXITING after error: status=%q, want error preserved", s.status)
	}
}
