//go:build darwin

package daemon

import (
	"fmt"
	"os/exec"
	"strings"
)

// assignAddresses assigns IP addresses to the named interface on macOS.
func assignAddresses(iface string, addresses []string) error {
	for _, addr := range addresses {
		parts := strings.SplitN(addr, "/", 2)
		if len(parts) != 2 {
			continue
		}
		ip := strings.TrimSpace(parts[0])
		out, err := exec.Command("ifconfig", iface, "inet", ip, ip, "alias").CombinedOutput()
		if err != nil {
			return fmt.Errorf("ifconfig add %s: %s", addr, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// addRoutes adds routes for the given CIDRs through the named interface.
// For 0.0.0.0/0 (full tunnel) we use two /1 routes instead of replacing the
// default route — this matches wg-quick behaviour and ensures the original
// default is still present when the tunnel is torn down.
func addRoutes(iface string, cidrs []string, gateway string) error {
	for _, cidr := range cidrs {
		switch cidr {
		case "0.0.0.0/0":
			exec.Command("route", "-q", "-n", "add", "-inet", "0.0.0.0/1", "-interface", iface).Run()
			exec.Command("route", "-q", "-n", "add", "-inet", "128.0.0.0/1", "-interface", iface).Run()
		case "::/0":
			exec.Command("route", "-q", "-n", "add", "-inet6", "::/1", "-interface", iface).Run()
			exec.Command("route", "-q", "-n", "add", "-inet6", "8000::/1", "-interface", iface).Run()
		default:
			if strings.Contains(cidr, ":") {
				exec.Command("route", "-q", "-n", "add", "-inet6", cidr, "-interface", iface).Run()
			} else {
				exec.Command("route", "-q", "-n", "add", "-inet", cidr, "-interface", iface).Run()
			}
		}
	}
	return nil
}

// addEndpointRoute adds a host route for the WireGuard peer endpoint via the
// real default gateway. This prevents the UDP packets from being routed into
// the tunnel itself (routing loop) when full-tunnel mode is active.
func addEndpointRoute(endpointHost string) {
	gw := defaultGatewayDarwin()
	if gw == "" || endpointHost == "" {
		return
	}
	exec.Command("route", "-q", "-n", "add", "-host", endpointHost, gw).Run()
}

// removeEndpointRoute removes the endpoint protection host route.
func removeEndpointRoute(endpointHost string) {
	if endpointHost == "" {
		return
	}
	exec.Command("route", "-q", "-n", "delete", "-host", endpointHost).Run()
}

// removeRoutes removes routes previously added by addRoutes.
func removeRoutes(iface string, cidrs []string, gateway string) error {
	for _, cidr := range cidrs {
		switch cidr {
		case "0.0.0.0/0":
			exec.Command("route", "-q", "-n", "delete", "-inet", "0.0.0.0/1", "-interface", iface).Run()
			exec.Command("route", "-q", "-n", "delete", "-inet", "128.0.0.0/1", "-interface", iface).Run()
		case "::/0":
			exec.Command("route", "-q", "-n", "delete", "-inet6", "::/1", "-interface", iface).Run()
			exec.Command("route", "-q", "-n", "delete", "-inet6", "8000::/1", "-interface", iface).Run()
		default:
			if strings.Contains(cidr, ":") {
				exec.Command("route", "-q", "-n", "delete", "-inet6", cidr, "-interface", iface).Run()
			} else {
				exec.Command("route", "-q", "-n", "delete", "-inet", cidr, "-interface", iface).Run()
			}
		}
	}
	return nil
}

// defaultGatewayDarwin returns the current IPv4 default gateway.
func defaultGatewayDarwin() string {
	out, err := exec.Command("route", "-n", "get", "default").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		}
	}
	return ""
}

// setDNS configures DNS on macOS via scutil.
func setDNS(iface string, servers []string) error {
	if len(servers) == 0 {
		return nil
	}
	dnsArgs := strings.Join(servers, " ")
	script := fmt.Sprintf(`
open
d.init
d.add ServerAddresses * %s
set State:/Network/Service/proidentity-%s/DNS
quit
`, dnsArgs, iface)
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scutil dns: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// clearInterface cleans up any lingering DNS state for the interface.
// Routes and the utun device itself are removed automatically by the kernel
// when the TUN file descriptor is closed.
func clearInterface(iface string) {
	script := fmt.Sprintf(`
open
remove State:/Network/Service/proidentity-%s/DNS
quit
`, iface)
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader(script)
	cmd.Run()
}
