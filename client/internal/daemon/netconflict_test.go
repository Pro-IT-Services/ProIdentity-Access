package daemon

import (
	"reflect"
	"testing"
)

func TestParsePushReply(t *testing.T) {
	line := "2026-09-28 19:40:02 PUSH: Received control message: 'PUSH_REPLY,route 10.20.0.0 255.255.0.0,route-gateway 172.16.0.1,topology subnet,ping 10,ping-restart 120,ifconfig 172.16.0.11 255.255.255.0,peer-id 0,cipher AES-256-GCM'"
	n, ok := parsePushReply(line)
	if !ok {
		t.Fatal("not recognised")
	}
	want := []string{"10.20.0.0/16", "172.16.0.0/24"}
	if got := n.prefixStrings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if n.DefaultRoute {
		t.Fatal("no redirect-gateway pushed")
	}
}

func TestPushReplyDefaultRouteAndNet30(t *testing.T) {
	n, _ := parsePushReply("PUSH: Received control message: 'PUSH_REPLY,redirect-gateway def1,topology net30,ifconfig 10.8.0.6 10.8.0.5,route-ipv6 2001:db8:1::/64'")
	if !n.DefaultRoute {
		t.Fatal("redirect-gateway not seen")
	}
	want := []string{"all traffic", "10.8.0.6/32", "2001:db8:1::/64"}
	if got := n.prefixStrings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSameSubnetFromTwoSitesConflicts(t *testing.T) {
	a, _ := parsePushReply("PUSH_REPLY,ifconfig 172.16.0.11 255.255.255.0")
	b, _ := parsePushReply("PUSH_REPLY,ifconfig 172.16.0.12 255.255.255.0,route 192.168.50.0 255.255.255.0")
	what, ok := overlap(a, b)
	if !ok || what != "172.16.0.0/24" {
		t.Fatalf("got %q %v", what, ok)
	}
}

func TestNestedRangesConflict(t *testing.T) {
	var a, b netSet
	a.addCIDR("10.0.0.0/8")
	b.addCIDR("10.20.30.0/24")
	if what, ok := overlap(a, b); !ok || what != "10.0.0.0/8" {
		t.Fatalf("got %q %v", what, ok)
	}
}

func TestDistinctNetworksDoNotConflict(t *testing.T) {
	a, _ := parsePushReply("PUSH_REPLY,ifconfig 172.16.0.11 255.255.255.0")
	b := wireGuardNetworks([]string{"10.99.1.5/32"}, []string{"10.99.1.0/24", "192.168.211.0/24"})
	if what, ok := overlap(a, b); ok {
		t.Fatalf("unexpected conflict on %q", what)
	}
}

func TestOneFullTunnelDoesNotBlockSplitTunnel(t *testing.T) {
	full := wireGuardNetworks([]string{"10.99.1.5/32"}, []string{"0.0.0.0/0", "::/0"})
	split := profileNetworks("route 192.168.1.0 255.255.255.0\n")
	if _, ok := overlap(full, split); ok {
		t.Fatal("a full tunnel shouldn't block a more specific split tunnel")
	}
	other := profileNetworks("redirect-gateway def1\n")
	if what, ok := overlap(full, other); !ok || what == "" {
		t.Fatal("two full tunnels must conflict")
	}
}

func TestProfileNetworksIgnoresComments(t *testing.T) {
	n := profileNetworks("client\n# route 10.0.0.0 255.0.0.0\n;route 10.1.0.0 255.255.0.0\nroute 192.168.7.0 255.255.255.0 vpn_gateway\n")
	if got := n.prefixStrings(); !reflect.DeepEqual(got, []string{"192.168.7.0/24"}) {
		t.Fatalf("got %v", got)
	}
}
