package main

import (
	"strings"
	"testing"
)

func TestWithPersistentKeepalive(t *testing.T) {
	in := "[Interface]\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = x\nEndpoint = h:51820\n\n[Peer]\nPublicKey = y\nPersistentKeepalive = 10\n"
	out := withPersistentKeepalive(in)
	if n := strings.Count(out, "PersistentKeepalive"); n != 2 {
		t.Fatalf("want 2 keepalive lines, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "Endpoint = h:51820\nPersistentKeepalive = 25\n\n[Peer]") {
		t.Fatalf("keepalive not added to the first peer:\n%s", out)
	}
	if !strings.Contains(out, "PersistentKeepalive = 10") {
		t.Fatalf("existing keepalive changed:\n%s", out)
	}
	if got := withPersistentKeepalive("[Interface]\nAddress = 10.0.0.2/32\n"); strings.Contains(got, "Keepalive") {
		t.Fatalf("added keepalive outside [Peer]:\n%s", got)
	}
}
