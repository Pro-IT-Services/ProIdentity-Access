package daemon

import (
	"fmt"

	"wg-client/internal/ipc"
)

// findConflict reports a live connection (WireGuard or OpenVPN, any user on
// this computer, routes being machine-wide) that already uses one of want's
// networks. except is the connection being started.
func (m *TunnelManager) findConflict(except, ownerID string, want netSet) (string, bool) {
	if want.empty() {
		return "", false
	}
	for _, l := range m.liveNetworks(except) {
		what, ok := overlap(want, l.nets)
		if !ok {
			continue
		}
		who := fmt.Sprintf("%q", l.name)
		if l.ownerID != ownerID {
			who = "another VPN connection on this computer" // don't reveal other users' connections
		}
		return fmt.Sprintf("Can't connect: %s is already connected and also uses %s. Disconnect it first.", who, what), true
	}
	return "", false
}

func (m *TunnelManager) liveNetworks(except string) []liveNet {
	out := m.openvpn.liveNetworks(except)

	m.mu.RLock()
	tunnels := make(map[string]*Tunnel, len(m.tunnels))
	for id, t := range m.tunnels {
		tunnels[id] = t
	}
	m.mu.RUnlock()

	for id, t := range tunnels {
		if id == except {
			continue
		}
		info := t.Info()
		if info.Status != ipc.StatusConnected && info.Status != ipc.StatusConnecting {
			continue
		}
		out = append(out, liveNet{id: id, name: info.Name, ownerID: info.OwnerID, nets: tunnelNetworks(info)})
	}
	return out
}

func tunnelNetworks(info ipc.TunnelInfo) netSet {
	var allowed []string
	for _, p := range info.Peers {
		allowed = append(allowed, p.AllowedIPs...)
	}
	return wireGuardNetworks(info.Addresses, allowed)
}
