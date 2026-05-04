//go:build windows

package daemon

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
)

// assignAddresses assigns IP addresses to the named network interface on Windows.
func assignAddresses(iface string, addresses []string) error {
	// Remove existing addresses first
	exec.Command("netsh", "interface", "ip", "set", "address", iface, "static", "none").Run()

	for i, addr := range addresses {
		ip, mask, err := splitCIDR(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q: %w", addr, err)
		}
		var args []string
		if i == 0 {
			args = []string{"interface", "ip", "set", "address", iface, "static", ip, mask}
		} else {
			args = []string{"interface", "ip", "add", "address", iface, ip, mask}
		}
		if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("set address %s: %s", addr, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// ifIndex returns the Windows interface index for the named interface,
// as a string suitable for appending to a route ADD command ("IF <idx>").
// Returns "" if the interface cannot be found.
func ifIndex(iface string) string {
	ni, err := net.InterfaceByName(iface)
	if err != nil {
		log.Printf("warn: InterfaceByName(%q): %v", iface, err)
		return ""
	}
	return fmt.Sprintf("%d", ni.Index)
}

// addRoutes adds routes for the given CIDRs through the WireGuard gateway IP.
// gateway is the local WireGuard interface IP (e.g. 10.8.0.5).
// For 0.0.0.0/0 (full tunnel) it splits into two /1 routes and protects
// the WireGuard endpoint so its UDP traffic still uses the real network.
func addRoutes(iface string, cidrs []string, gateway string) error {
	idx := ifIndex(iface)
	for _, cidr := range cidrs {
		switch cidr {
		case "::/0":
			// Skip IPv6 default — not handled
			continue
		case "0.0.0.0/0":
			// Full tunnel: use two /1 routes that are more specific than any real
			// default gateway, so they win without replacing it.
			for _, r := range []string{"0.0.0.0 MASK 128.0.0.0", "128.0.0.0 MASK 128.0.0.0"} {
				parts := strings.Fields(r)
				args := append([]string{"ADD"}, parts...)
				args = append(args, gateway, "METRIC", "5")
				if idx != "" {
					args = append(args, "IF", idx)
				}
				if out, err := exec.Command("route", args...).CombinedOutput(); err != nil {
					log.Printf("warn: route ADD %s via %s IF %s: %v: %s", r, gateway, idx, err, strings.TrimSpace(string(out)))
				}
			}
		default:
			ip, mask, err := splitCIDR(cidr)
			if err != nil {
				continue
			}
			args := []string{"ADD", ip, "MASK", mask, gateway, "METRIC", "5"}
			if idx != "" {
				args = append(args, "IF", idx)
			}
			if out, err := exec.Command("route", args...).CombinedOutput(); err != nil {
				log.Printf("warn: route ADD %s via %s IF %s: %v: %s", cidr, gateway, idx, err, strings.TrimSpace(string(out)))
			}
		}
	}
	return nil
}

// addEndpointRoute adds a specific host route for the WireGuard server endpoint
// via the original default gateway. This prevents the WireGuard UDP traffic from
// being routed into the tunnel itself (routing loop) in full-tunnel mode.
func addEndpointRoute(endpointHost string) {
	gw := defaultGateway()
	if gw == "" || endpointHost == "" {
		return
	}
	if out, err := exec.Command("route", "ADD", endpointHost, "MASK", "255.255.255.255", gw).CombinedOutput(); err != nil {
		log.Printf("warn: endpoint route ADD %s via %s: %v: %s", endpointHost, gw, err, strings.TrimSpace(string(out)))
	}
}

// removeEndpointRoute removes the endpoint protection route.
func removeEndpointRoute(endpointHost string) {
	exec.Command("route", "DELETE", endpointHost, "MASK", "255.255.255.255").Run()
}

// removeRoutes removes routes previously added by addRoutes.
func removeRoutes(iface string, cidrs []string, gateway string) error {
	for _, cidr := range cidrs {
		switch cidr {
		case "::/0":
			continue
		case "0.0.0.0/0":
			exec.Command("route", "DELETE", "0.0.0.0", "MASK", "128.0.0.0", gateway).Run()
			exec.Command("route", "DELETE", "128.0.0.0", "MASK", "128.0.0.0", gateway).Run()
		default:
			ip, mask, err := splitCIDR(cidr)
			if err != nil {
				continue
			}
			exec.Command("route", "DELETE", ip, "MASK", mask, gateway).Run()
		}
	}
	return nil
}

// setDNS sets DNS servers for the named interface on Windows.
func setDNS(iface string, servers []string) error {
	if len(servers) == 0 {
		return nil
	}
	args := []string{"interface", "ip", "set", "dns", iface, "static", servers[0]}
	if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("set dns: %s", strings.TrimSpace(string(out)))
	}
	for _, s := range servers[1:] {
		args = []string{"interface", "ip", "add", "dns", iface, s}
		exec.Command("netsh", args...).Run()
	}
	return nil
}

// defaultGateway returns the current IPv4 default gateway by parsing `route PRINT 0.0.0.0`.
func defaultGateway() string {
	out, err := exec.Command("route", "PRINT", "0.0.0.0").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// Active Routes section has lines: Network Dest  Netmask  Gateway  Interface  Metric
		// The default route is: 0.0.0.0  0.0.0.0  <gateway>  ...
		if len(fields) >= 3 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			gw := fields[2]
			// Skip "On-link" entries
			if gw != "On-link" && strings.Count(gw, ".") == 3 {
				return gw
			}
		}
	}
	return ""
}

// splitCIDR splits "10.0.0.2/24" into IP and dotted-decimal mask.
func splitCIDR(cidr string) (ip, mask string, err error) {
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("missing prefix length")
	}
	ip = strings.TrimSpace(parts[0])
	var bits int
	if _, err = fmt.Sscanf(parts[1], "%d", &bits); err != nil {
		return "", "", fmt.Errorf("invalid prefix: %w", err)
	}
	mask = prefixToMask(bits)
	return ip, mask, nil
}

// clearInterface is a no-op on Windows; netsh handles cleanup when the adapter is removed.
func clearInterface(iface string) {}

func prefixToMask(bits int) string {
	if bits < 0 {
		bits = 0
	}
	if bits > 32 {
		bits = 32
	}
	var m uint32
	if bits == 32 {
		m = 0xffffffff
	} else {
		m = ^uint32((1 << (32 - bits)) - 1)
	}
	return fmt.Sprintf("%d.%d.%d.%d", (m>>24)&0xff, (m>>16)&0xff, (m>>8)&0xff, m&0xff)
}
