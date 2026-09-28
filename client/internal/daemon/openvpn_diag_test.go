package daemon

import (
	"strings"
	"testing"
)

func TestDiagnoseOpenVPNLogLines(t *testing.T) {
	cases := []struct {
		line  string
		want  string
		fatal bool
	}{
		{"2026-09-28 19:05:01 AUTH: Received control message: AUTH_FAILED", "rejected the username or password", true},
		{"2026-09-28 19:05:01 All ovpn-dco adapters on this system are currently in use or disabled.", "No free VPN network adapter", true},
		{"2026-09-28 19:05:01 VERIFY ERROR: depth=0, error=certificate has expired", "certificate couldn't be verified", true},
		{"2026-09-28 19:05:01 RESOLVE: Cannot resolve host address: vpn.example.com:1194 (No such host is known.)", "couldn't be resolved", false},
		{"2026-09-28 19:05:01 TLS Error: TLS key negotiation failed to occur within 60 seconds (check your network connectivity)", "No response from the VPN server", false},
		{"2026-09-28 19:05:01 Options error: Unrecognized option or missing or extra parameter(s) in x.ovpn:12: comp-lzo", "can't use: Unrecognized option", true},
	}
	for _, c := range cases {
		d, ok := diagnose(c.line)
		if !ok || !strings.Contains(d.message, c.want) || d.fatal != c.fatal {
			t.Errorf("%q → %+v (ok=%v), want %q fatal=%v", c.line, d, ok, c.want, c.fatal)
		}
	}
	if _, ok := diagnose("2026-09-28 19:05:01 Initialization Sequence Completed"); ok {
		t.Error("success line diagnosed as a failure")
	}
}

func TestQuotedManagementName(t *testing.T) {
	if got := quoted(">NEED-OK:Need 'token-insertion-request' confirmation MSG:Please insert"); got != "token-insertion-request" {
		t.Fatalf("got %q", got)
	}
	if got := quoted(">NEED-OK:no quotes"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTrimLogPrefix(t *testing.T) {
	if got := trimLogPrefix("2026-09-28 19:05:01 TLS Error: handshake failed"); got != "TLS Error: handshake failed" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteAttemptNamesTheServer(t *testing.T) {
	addr, proto := remoteAttempt("2026-09-28 19:15:15 Attempting to establish TCP connection with [AF_INET]203.0.113.10:1194")
	if addr != "203.0.113.10:1194" || proto != "TCP" {
		t.Fatalf("got %q %q", addr, proto)
	}
	addr, proto = remoteAttempt("2026-09-28 19:15:15 UDP link remote: [AF_INET6]2001:db8::1:1194")
	if addr != "2001:db8::1:1194" || proto != "UDP" {
		t.Fatalf("got %q %q", addr, proto)
	}
	if a, _ := remoteAttempt("2026-09-28 19:15:15 Initialization Sequence Completed"); a != "" {
		t.Fatal("unexpected match")
	}
}
