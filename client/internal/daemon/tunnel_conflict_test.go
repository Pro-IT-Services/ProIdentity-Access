package daemon

import (
	"strings"
	"testing"

	"wg-client/internal/ipc"
)

// managerWithLiveSession: one connected OpenVPN session using 172.16.0.0/24,
// and conflict checks wired like the TunnelManager does (OpenVPN only here).
func managerWithLiveSession(owner string) *OpenVPNManager {
	m := NewOpenVPNManager(nil)
	var live netSet
	live.addCIDR("172.16.0.0/24")
	m.sessions["assigned:office-a"] = &ovpnSession{
		id: "assigned:office-a", ownerID: owner, name: "Office A",
		status: ipc.StatusConnected, networks: live,
	}
	tm := &TunnelManager{tunnels: map[string]*Tunnel{}, openvpn: m}
	m.conflict = tm.findConflict
	return m
}

func TestPushedNetworksOverlappingLiveVPNStopTheNewOne(t *testing.T) {
	m := managerWithLiveSession("S-1-5-21-me")
	sess := &ovpnSession{id: "assigned:office-b", ownerID: "S-1-5-21-me", name: "Office B", status: ipc.StatusConnecting}
	m.sessions[sess.id] = sess

	pushed, _ := parsePushReply("PUSH_REPLY,ifconfig 172.16.0.11 255.255.255.0")
	m.onPushedNetworks(sess, pushed)

	st := sess.snapshot()
	if st.Status != ipc.StatusError || !strings.Contains(st.Error, "Office A") || !strings.Contains(st.Error, "172.16.0.0/24") {
		t.Fatalf("got %s %q", st.Status, st.Error)
	}
	// openvpn's own shutdown error must not replace the explanation.
	sess.setError("OpenVPN stopped: NETSH: command failed")
	if got := sess.snapshot().Error; !strings.Contains(got, "172.16.0.0/24") {
		t.Fatalf("reason overwritten: %q", got)
	}
	// Remembered: the next attempt is refused before openvpn starts.
	m.mu.Lock()
	known := m.known["assigned:office-b"]
	m.mu.Unlock()
	if msg, bad := m.checkConflict("assigned:office-b", "S-1-5-21-me", known); !bad || !strings.Contains(msg, "Disconnect it first") {
		t.Fatalf("pre-check didn't refuse: %q", msg)
	}
}

func TestOtherUsersConnectionIsNotNamed(t *testing.T) {
	m := managerWithLiveSession("S-1-5-21-someone-else")
	var want netSet
	want.addCIDR("172.16.0.0/24")
	msg, bad := m.checkConflict("assigned:office-b", "S-1-5-21-me", want)
	if !bad || strings.Contains(msg, "Office A") || !strings.Contains(msg, "another VPN connection") {
		t.Fatalf("got %q", msg)
	}
}

func TestNonOverlappingVPNsCanRunTogether(t *testing.T) {
	m := managerWithLiveSession("S-1-5-21-me")
	var want netSet
	want.addCIDR("10.60.0.0/24")
	if msg, bad := m.checkConflict("assigned:other", "S-1-5-21-me", want); bad {
		t.Fatalf("unexpected conflict: %q", msg)
	}
}
