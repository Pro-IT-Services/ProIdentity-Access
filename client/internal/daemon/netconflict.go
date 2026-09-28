package daemon

import (
	"net"
	"net/netip"
	"strings"
)

// Overlapping VPN networks.
//
// Routes are machine-wide: if two VPNs hand out the same range (common when
// several sites use a default like 172.16.0.0/24), the second one can't set
// its address or routes ("NETSH: command failed" on Windows) or silently
// steals the first one's traffic. The service refuses the second connection
// with a clear reason instead.

// netSet is what one connection claims: specific networks plus whether it
// takes over the default route (all traffic).
type netSet struct {
	Prefixes     []netip.Prefix
	DefaultRoute bool
}

func (n netSet) empty() bool { return len(n.Prefixes) == 0 && !n.DefaultRoute }

// add records a network; default routes (/0, and OpenVPN's def1 halves
// 0.0.0.0/1 + 128.0.0.0/1) count as "all traffic" instead.
func (n *netSet) add(p netip.Prefix) {
	p = p.Masked()
	if p.Bits() <= 1 {
		n.DefaultRoute = true
		return
	}
	for _, have := range n.Prefixes {
		if have == p {
			return
		}
	}
	n.Prefixes = append(n.Prefixes, p)
}

// addCIDR parses "a.b.c.d/n", "a.b.c.d" (host) or IPv6 forms.
func (n *netSet) addCIDR(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if p, err := netip.ParsePrefix(s); err == nil {
		n.add(p)
		return
	}
	if a, err := netip.ParseAddr(s); err == nil {
		n.add(netip.PrefixFrom(a, a.BitLen()))
	}
}

// addMasked parses OpenVPN's "network netmask" pairs.
func (n *netSet) addMasked(ip, mask string) {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return
	}
	bits := a.BitLen()
	if m := net.ParseIP(mask).To4(); m != nil && a.Is4() {
		ones, total := net.IPMask(m).Size()
		if total == 0 { // not a contiguous netmask (e.g. a p2p peer address)
			ones = 32
		}
		bits = ones
	}
	n.add(netip.PrefixFrom(a, bits))
}

// overlap returns the first shared network between a and b, or "all traffic"
// when both take the default route.
func overlap(a, b netSet) (string, bool) {
	if a.DefaultRoute && b.DefaultRoute {
		return "all traffic (both send everything through the VPN)", true
	}
	for _, x := range a.Prefixes {
		for _, y := range b.Prefixes {
			if x.Overlaps(y) {
				if x.Bits() <= y.Bits() {
					return x.String(), true
				}
				return y.String(), true
			}
		}
	}
	return "", false
}

// parsePushReply extracts the networks from an OpenVPN PUSH_REPLY log line:
//
//	PUSH: Received control message: 'PUSH_REPLY,route 10.1.0.0 255.255.0.0,
//	  route-gateway 172.16.0.1,topology subnet,ifconfig 172.16.0.11 255.255.255.0,...'
func parsePushReply(line string) (netSet, bool) {
	i := strings.Index(line, "PUSH_REPLY,")
	if i < 0 {
		return netSet{}, false
	}
	body := strings.TrimRight(line[i+len("PUSH_REPLY,"):], "' ")
	var n netSet
	for _, opt := range strings.Split(body, ",") {
		addOpenVPNOption(&n, strings.Fields(opt))
	}
	return n, true
}

// profileNetworks reads static networks from a profile (route / ifconfig /
// redirect-gateway lines), known before connecting.
func profileNetworks(cfg string) netSet {
	var n netSet
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		addOpenVPNOption(&n, strings.Fields(line))
	}
	return n
}

func addOpenVPNOption(n *netSet, f []string) {
	if len(f) == 0 {
		return
	}
	switch f[0] {
	case "route":
		if len(f) >= 3 {
			n.addMasked(f[1], f[2])
		} else if len(f) == 2 {
			n.addMasked(f[1], "255.255.255.255")
		}
	case "route-ipv6", "ifconfig-ipv6":
		if len(f) >= 2 {
			n.addCIDR(f[1])
		}
	case "ifconfig":
		if len(f) >= 3 {
			n.addMasked(f[1], f[2])
		}
	case "redirect-gateway":
		n.DefaultRoute = true
	}
}

// wireGuardNetworks: interface addresses plus peers' AllowedIPs.
func wireGuardNetworks(addresses, allowedIPs []string) netSet {
	var n netSet
	for _, a := range addresses {
		n.addCIDR(a)
	}
	for _, a := range allowedIPs {
		n.addCIDR(a)
	}
	return n
}

// prefixStrings renders a set for the app.
func (n netSet) prefixStrings() []string {
	out := make([]string, 0, len(n.Prefixes)+1)
	if n.DefaultRoute {
		out = append(out, "all traffic")
	}
	for _, p := range n.Prefixes {
		out = append(out, p.String())
	}
	return out
}

// merge combines two sets.
func (n netSet) merge(o netSet) netSet {
	out := netSet{DefaultRoute: n.DefaultRoute || o.DefaultRoute}
	for _, p := range n.Prefixes {
		out.add(p)
	}
	for _, p := range o.Prefixes {
		out.add(p)
	}
	return out
}
