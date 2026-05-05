package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"wg-client/internal/config"
	"wg-client/internal/ipc"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

// TunnelStats holds live traffic statistics for a tunnel.
type TunnelStats struct {
	RxBytes       int64
	TxBytes       int64
	LastHandshake int64 // Unix seconds
}

// Tunnel manages a single WireGuard tunnel instance.
type Tunnel struct {
	Config *config.TunnelConfig

	mu     sync.RWMutex
	status ipc.TunnelStatus
	errMsg string
	dev    *device.Device
	tdev   tun.Device
}

// NewTunnel creates a Tunnel for the given configuration.
func NewTunnel(cfg *config.TunnelConfig) *Tunnel {
	return &Tunnel{
		Config: cfg,
		status: ipc.StatusDisconnected,
	}
}

// Status returns the current status of the tunnel.
func (t *Tunnel) Status() ipc.TunnelStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// Info returns UI-safe tunnel metadata without WireGuard key material.
func (t *Tunnel) Info() ipc.TunnelInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	peers := make([]ipc.PeerInfo, len(t.Config.Peers))
	for i, p := range t.Config.Peers {
		peers[i] = ipc.PeerInfo{
			Endpoint:            p.Endpoint,
			AllowedIPs:          p.AllowedIPs,
			PersistentKeepalive: p.PersistentKeepalive,
		}
	}

	return ipc.TunnelInfo{
		ID:         t.Config.ID,
		OwnerID:    t.Config.OwnerID,
		Name:       t.Config.Name,
		Status:     t.status,
		Addresses:  t.Config.Interface.Addresses,
		DNS:        t.Config.Interface.DNS,
		MTU:        t.Config.Interface.MTU,
		ListenPort: t.Config.Interface.ListenPort,
		Peers:      peers,
		Error:      t.errMsg,
	}
}

// Start brings up the WireGuard tunnel.
func (t *Tunnel) Start() error {
	t.mu.Lock()
	if t.status == ipc.StatusConnected || t.status == ipc.StatusConnecting {
		t.mu.Unlock()
		return nil
	}
	t.status = ipc.StatusConnecting
	t.errMsg = ""
	t.mu.Unlock()

	tunName := sanitizeName(t.Config.Name)
	// macOS requires utun[0-9]+ interface names; pick the next free one.
	if runtime.GOOS == "darwin" {
		tunName = nextUtunName()
	}
	mtu := t.Config.Interface.MTU
	if mtu <= 0 {
		mtu = 1420
	}
	resolveTunnelPeerEndpoints(t.Config)

	// 1. Create platform TUN device
	tdev, err := tun.CreateTUN(tunName, mtu)
	if err != nil {
		return t.setError(fmt.Errorf("create TUN %q: %w", tunName, err))
	}

	// 2. Create WireGuard device
	logger := device.NewLogger(device.LogLevelError, fmt.Sprintf("[%s] ", t.Config.Name))
	dev := device.NewDevice(tdev, conn.NewDefaultBind(), logger)

	// 3. Configure via UAPI
	uapiCfg := t.Config.GenerateUAPI()
	if err := dev.IpcSetOperation(strings.NewReader(uapiCfg)); err != nil {
		dev.Close()
		tdev.Close()
		return t.setError(fmt.Errorf("configure WireGuard: %w", err))
	}

	// 4. Bring up
	if err := dev.Up(); err != nil {
		dev.Close()
		tdev.Close()
		return t.setError(fmt.Errorf("device up: %w", err))
	}

	// Get the real interface name assigned by the OS (may differ from requested)
	realName, err := tdev.Name()
	if err != nil {
		realName = tunName
	}

	// 5. Assign IP addresses on the interface
	if err := assignAddresses(realName, t.Config.Interface.Addresses); err != nil {
		dev.Down()
		dev.Close()
		tdev.Close()
		return t.setError(fmt.Errorf("assign addresses: %w", err))
	}

	// Derive local WireGuard IP for use as route gateway (Windows needs this)
	gateway := localIP(t.Config.Interface.Addresses)

	// 6a. For full-tunnel mode, protect peer endpoints so their UDP traffic
	//     still uses the real network (prevents routing loop on Windows).
	allowedIPs := t.Config.AllPeerAllowedIPs()
	if hasFullTunnel(allowedIPs) {
		for _, peer := range t.Config.Peers {
			if host := endpointHost(peer.Endpoint); host != "" {
				addEndpointRoute(host)
			}
		}
	}

	// 6b. Add routes for all peer AllowedIPs
	if err := addRoutes(realName, allowedIPs, gateway); err != nil {
		log.Printf("warn: add routes for %s: %v", t.Config.Name, err)
	}

	// 7. Set DNS
	if len(t.Config.Interface.DNS) > 0 {
		if err := setDNS(realName, t.Config.Interface.DNS); err != nil {
			log.Printf("warn: set DNS for %s: %v", t.Config.Name, err)
		}
	}

	t.mu.Lock()
	t.dev = dev
	t.tdev = tdev
	t.status = ipc.StatusConnected
	t.mu.Unlock()

	// Watch for device errors
	go func() {
		<-dev.Wait()
		t.mu.Lock()
		if t.status == ipc.StatusConnected {
			t.status = ipc.StatusDisconnected
		}
		t.mu.Unlock()
	}()

	return nil
}

// Stop tears down the WireGuard tunnel.
func (t *Tunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == ipc.StatusDisconnected {
		return nil
	}

	realName := sanitizeName(t.Config.Name)
	if t.tdev != nil {
		name, err := t.tdev.Name()
		if err == nil {
			realName = name
		}
	}

	// Remove routes
	allowedIPs := t.Config.AllPeerAllowedIPs()
	gateway := localIP(t.Config.Interface.Addresses)
	_ = removeRoutes(realName, allowedIPs, gateway)

	// Remove endpoint protection routes (only relevant for full-tunnel)
	if hasFullTunnel(allowedIPs) {
		for _, peer := range t.Config.Peers {
			if host := endpointHost(peer.Endpoint); host != "" {
				removeEndpointRoute(host)
			}
		}
	}

	if t.dev != nil {
		t.dev.Down()
		t.dev.Close()
		t.dev = nil
	}
	if t.tdev != nil {
		t.tdev.Close()
		t.tdev = nil
	}

	// Clean up any lingering DNS/interface state (e.g. scutil DNS on macOS).
	clearInterface(realName)

	t.status = ipc.StatusDisconnected
	return nil
}

// Stats reads current byte counts and last handshake time from the device.
func (t *Tunnel) Stats() (TunnelStats, error) {
	t.mu.RLock()
	dev := t.dev
	t.mu.RUnlock()

	if dev == nil {
		return TunnelStats{}, fmt.Errorf("tunnel not connected")
	}

	var sb strings.Builder
	if err := dev.IpcGetOperation(&sb); err != nil {
		return TunnelStats{}, err
	}

	return parseUAPIStats(sb.String()), nil
}

func (t *Tunnel) setError(err error) error {
	t.mu.Lock()
	t.status = ipc.StatusError
	t.errMsg = err.Error()
	t.mu.Unlock()
	return err
}

// localIP extracts the bare IP from the first address CIDR (e.g. "10.8.0.5/32" → "10.8.0.5").
func localIP(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	parts := strings.SplitN(addresses[0], "/", 2)
	return strings.TrimSpace(parts[0])
}

// hasFullTunnel reports whether any of the CIDRs is the IPv4 default route.
func hasFullTunnel(cidrs []string) bool {
	for _, c := range cidrs {
		if c == "0.0.0.0/0" {
			return true
		}
	}
	return false
}

// endpointHost extracts the host part from "host:port", returning "" on error.
func endpointHost(endpoint string) string {
	host, _, err := net.SplitHostPort(endpoint)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	idx := strings.LastIndex(endpoint, ":")
	if idx < 0 {
		return strings.Trim(endpoint, "[]")
	}
	return strings.Trim(endpoint[:idx], "[]")
}

func resolveTunnelPeerEndpoints(cfg *config.TunnelConfig) {
	for i := range cfg.Peers {
		host, port, err := net.SplitHostPort(cfg.Peers[i].Endpoint)
		if err != nil || host == "" || port == "" {
			continue
		}
		host = strings.Trim(host, "[]")
		if net.ParseIP(host) != nil {
			continue
		}
		ip, err := resolvePeerEndpointHost(host)
		if err != nil {
			log.Printf("warn: resolve endpoint %s: %v", host, err)
			continue
		}
		cfg.Peers[i].Endpoint = net.JoinHostPort(ip, port)
	}
}

func resolvePeerEndpointHost(host string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, addr := range ips {
		if v4 := addr.IP.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	for _, addr := range ips {
		if ip := addr.IP.To16(); ip != nil {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("no usable IP address")
}

// sanitizeName returns a safe interface name (max 15 chars on Linux, no spaces).
func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	if len(name) > 15 {
		name = name[:15]
	}
	return name
}

// nextUtunName returns the lowest utunN name not already in use on macOS.
func nextUtunName() string {
	ifaces, _ := net.Interfaces()
	used := make(map[string]bool, len(ifaces))
	for _, iface := range ifaces {
		used[iface.Name] = true
	}
	for i := 0; i < 256; i++ {
		name := fmt.Sprintf("utun%d", i)
		if !used[name] {
			return name
		}
	}
	return "utun0"
}

// parseUAPIStats extracts rx_bytes, tx_bytes, last_handshake_time_sec from UAPI get response.
func parseUAPIStats(uapi string) TunnelStats {
	var stats TunnelStats
	for _, line := range strings.Split(uapi, "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "rx_bytes":
			fmt.Sscanf(kv[1], "%d", &stats.RxBytes)
		case "tx_bytes":
			fmt.Sscanf(kv[1], "%d", &stats.TxBytes)
		case "last_handshake_time_sec":
			fmt.Sscanf(kv[1], "%d", &stats.LastHandshake)
		}
	}
	return stats
}
