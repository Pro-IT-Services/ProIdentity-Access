package firewall

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-iptables/iptables"
)

const chain = "WG-VPN"

// Rule describes a single firewall rule: destination CIDR and optional port list.
// Ports is a comma-separated list of ports/ranges, e.g. "80,443,8080-8090".
// If Ports is empty, all traffic to the CIDR is allowed.
type Rule struct {
	CIDR  string // destination CIDR, e.g. "10.0.1.0/24" or "192.168.1.5/32"
	Ports string // optional, e.g. "80,443"
}

// Manager handles per-session iptables rules in the WG-VPN FORWARD chain.
type Manager struct {
	ipt *iptables.IPTables
}

func NewManager() (*Manager, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, fmt.Errorf("iptables init: %w", err)
	}
	m := &Manager{ipt: ipt}
	if err := m.ensureChain(); err != nil {
		return nil, err
	}
	return m, nil
}

// NflogGroup is the netfilter NFLOG group number used for denied wg traffic.
// The userspace consumer in internal/denial subscribes to this group.
const NflogGroup = 100

// ensureChain sets up the WG-VPN chain and all permanent rules required for VPN forwarding.
func (m *Manager) ensureChain() error {
	// Enable IP forwarding
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
	}

	// Allow ESTABLISHED/RELATED packets in FORWARD so response traffic from
	// resources back to VPN clients is not dropped.
	if err := m.ipt.AppendUnique("filter", "FORWARD",
		"-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT",
	); err != nil {
		return fmt.Errorf("established rule: %w", err)
	}

	// Create WG-VPN chain for per-session rules
	exists, err := m.ipt.ChainExists("filter", chain)
	if err != nil {
		return fmt.Errorf("check chain: %w", err)
	}
	if !exists {
		if err := m.ipt.NewChain("filter", chain); err != nil {
			return fmt.Errorf("create chain: %w", err)
		}
	}

	// Hook WG-VPN into FORWARD for new connection decisions
	if err := m.ipt.AppendUnique("filter", "FORWARD", "-j", chain); err != nil {
		return fmt.Errorf("hook FORWARD→WG-VPN: %w", err)
	}

	// Anything coming IN on a wg* iface that wasn't ACCEPTed by WG-VPN is denied.
	// "wg+" is iptables wildcard for any interface whose name starts with "wg".
	// Two rules: NFLOG to userspace for visibility, then DROP. Rate-limit NFLOG
	// to avoid thundering during a port scan or misconfigured client.
	if err := m.ipt.AppendUnique("filter", "FORWARD",
		"-i", "wg+",
		"-m", "limit", "--limit", "60/min", "--limit-burst", "20",
		"-j", "NFLOG", "--nflog-group", fmt.Sprintf("%d", NflogGroup),
		"--nflog-prefix", "wg-deny:",
	); err != nil {
		return fmt.Errorf("nflog deny rule: %w", err)
	}
	if err := m.ipt.AppendUnique("filter", "FORWARD",
		"-i", "wg+", "-j", "DROP",
	); err != nil {
		return fmt.Errorf("drop deny rule: %w", err)
	}

	return nil
}

// EnsureSubnet adds a MASQUERADE rule for the given subnet if not already present.
func (m *Manager) EnsureSubnet(subnet string) error {
	return m.ipt.AppendUnique("nat", "POSTROUTING",
		"-s", subnet, "-j", "MASQUERADE",
	)
}

// RemoveSubnet removes the MASQUERADE rule for a subnet.
func (m *Manager) RemoveSubnet(subnet string) error {
	return m.ipt.DeleteIfExists("nat", "POSTROUTING",
		"-s", subnet, "-j", "MASQUERADE",
	)
}

// AddRules inserts ACCEPT rules for clientIP→each resource rule in WG-VPN.
// If a rule has ports, separate per-port rules are added using multiport match.
func (m *Manager) AddRules(clientIP string, rules []Rule) error {
	for _, r := range rules {
		if err := m.addRule(clientIP, r); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) addRule(clientIP string, r Rule) error {
	if r.Ports == "" {
		return m.ipt.AppendUnique("filter", chain,
			"-s", clientIP+"/32",
			"-d", r.CIDR,
			"-j", "ACCEPT",
		)
	}
	// With ports: allow TCP and UDP independently using multiport
	for _, proto := range []string{"tcp", "udp"} {
		ports := normalizePorts(r.Ports)
		if err := m.ipt.AppendUnique("filter", chain,
			"-s", clientIP+"/32",
			"-d", r.CIDR,
			"-p", proto,
			"-m", "multiport", "--dports", ports,
			"-j", "ACCEPT",
		); err != nil {
			return fmt.Errorf("add rule %s→%s:%s/%s: %w", clientIP, r.CIDR, r.Ports, proto, err)
		}
	}
	return nil
}

// AddFullTunnelRule adds a catch-all ACCEPT for a client (used when no specific resources).
func (m *Manager) AddFullTunnelRule(clientIP string) error {
	return m.ipt.AppendUnique("filter", chain,
		"-s", clientIP+"/32", "-j", "ACCEPT",
	)
}

// RemoveRules deletes ACCEPT rules for clientIP→each resource rule from WG-VPN.
func (m *Manager) RemoveRules(clientIP string, rules []Rule) error {
	var last error
	for _, r := range rules {
		if err := m.removeRule(clientIP, r); err != nil {
			last = err
		}
	}
	return last
}

func (m *Manager) removeRule(clientIP string, r Rule) error {
	if r.Ports == "" {
		return m.ipt.DeleteIfExists("filter", chain,
			"-s", clientIP+"/32",
			"-d", r.CIDR,
			"-j", "ACCEPT",
		)
	}
	var last error
	for _, proto := range []string{"tcp", "udp"} {
		ports := normalizePorts(r.Ports)
		if err := m.ipt.DeleteIfExists("filter", chain,
			"-s", clientIP+"/32",
			"-d", r.CIDR,
			"-p", proto,
			"-m", "multiport", "--dports", ports,
			"-j", "ACCEPT",
		); err != nil {
			last = fmt.Errorf("remove rule %s→%s:%s/%s: %w", clientIP, r.CIDR, r.Ports, proto, err)
		}
	}
	return last
}

// RemoveFullTunnelRule removes the catch-all ACCEPT rule for a client.
func (m *Manager) RemoveFullTunnelRule(clientIP string) error {
	return m.ipt.DeleteIfExists("filter", chain,
		"-s", clientIP+"/32", "-j", "ACCEPT",
	)
}

// FlushAll removes all rules from WG-VPN (used at shutdown/reset).
func (m *Manager) FlushAll() error {
	return m.ipt.ClearChain("filter", chain)
}

// normalizePorts strips spaces from a port spec so iptables won't reject it.
func normalizePorts(ports string) string {
	return strings.ReplaceAll(ports, " ", "")
}
