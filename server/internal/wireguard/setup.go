//go:build linux

package wireguard

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// SetupInterface creates a WireGuard interface, assigns the gateway IP, and brings it up.
// subnet is e.g. "10.8.1.0/24" — the gateway (.1) is derived automatically.
func SetupInterface(iface, subnet string, port int, privateKeyB64 string) error {
	// Parse subnet to get gateway IP
	gw, prefix, err := gatewayFromSubnet(subnet)
	if err != nil {
		return err
	}

	// Create the interface (ignore error if already exists)
	exec.Command("ip", "link", "add", iface, "type", "wireguard").Run()

	// Assign address. Different iproute2 versions emit different "already
	// present" messages: older "File exists", newer "Address already assigned".
	addr := fmt.Sprintf("%s/%d", gw, prefix)
	if out, err := exec.Command("ip", "address", "add", addr, "dev", iface).CombinedOutput(); err != nil {
		msg := strings.ToLower(string(out))
		if !strings.Contains(msg, "exists") && !strings.Contains(msg, "already") {
			return fmt.Errorf("assign address %s: %s", addr, strings.TrimSpace(string(out)))
		}
	}

	// Bring up
	if out, err := exec.Command("ip", "link", "set", iface, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("link up: %s", strings.TrimSpace(string(out)))
	}

	return nil
}

// TeardownInterface brings down and deletes a WireGuard interface.
func TeardownInterface(iface string) error {
	exec.Command("ip", "link", "set", iface, "down").Run()
	if out, err := exec.Command("ip", "link", "delete", iface).CombinedOutput(); err != nil {
		return fmt.Errorf("delete interface: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// gatewayFromSubnet parses "10.8.1.0/24" and returns gateway "10.8.1.1" and prefix 24.
func gatewayFromSubnet(subnet string) (string, int, error) {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", 0, fmt.Errorf("parse subnet %q: %w", subnet, err)
	}
	ones, _ := ipNet.Mask.Size()
	base := ipNet.IP.To4()
	if base == nil {
		return "", 0, fmt.Errorf("only IPv4 subnets supported")
	}
	gw := net.IP{base[0], base[1], base[2], base[3] + 1}
	return gw.String(), ones, nil
}
