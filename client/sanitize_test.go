package main

import (
	"encoding/json"
	"strings"
	"testing"

	"wg-client/internal/ipc"
)

// A stub for a not-yet-connected user config has no addresses or peers. The UI
// indexes into these arrays, so they must serialize as [] and never as null.
func TestSanitizeTunnelInfoNoNullArrays(t *testing.T) {
	stub := ipc.TunnelInfo{ID: "uconf:1", Name: "home", Status: ipc.StatusDisconnected}
	peerNoIPs := ipc.TunnelInfo{ID: "t2", Peers: []ipc.PeerInfo{{Endpoint: "203.0.113.1:51820"}}}

	for _, tun := range []*ipc.TunnelInfo{&stub, &peerNoIPs} {
		b, err := json.Marshal(sanitizeTunnelInfo(tun))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "null") {
			t.Errorf("tunnel %s serialized with null: %s", tun.ID, b)
		}
	}
}

func TestSanitizeTunnelInfoClearsKeys(t *testing.T) {
	tun := &ipc.TunnelInfo{PrivateKey: "secret", Peers: []ipc.PeerInfo{{PublicKey: "pub"}}}
	sanitizeTunnelInfo(tun)
	if tun.PrivateKey != "" || tun.Peers[0].PublicKey != "" {
		t.Fatalf("keys not cleared: %+v", tun)
	}
}
