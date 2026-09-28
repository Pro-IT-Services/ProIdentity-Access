//go:build windows

package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Network adapters for the bundled OpenVPN 2.7. The MSI installs the drivers
// (official ovpn-dco-win and tap-windows6 merge modules); adapters are created
// here on demand with the bundled tapctl, since this service runs as SYSTEM.
//
//   - TUN profiles use ovpn-dco (data channel offload) when the profile allows
//     it; OpenVPN falls back to tap-windows6 by itself otherwise.
//   - TAP profiles always use tap-windows6 (--disable-dco).
const (
	hwidDCO = "ovpn-dco"
	hwidTAP = `root\tap0901`

	adapterNameDCO = "ProIdentity OpenVPN"
	adapterNameTAP = "ProIdentity OpenVPN TAP"
)

var adapterMu sync.Mutex

func prepareOpenVPNAdapter(bin, devType string, inUse int) ([]string, error) {
	tapctl := filepath.Join(filepath.Dir(bin), "tapctl.exe")
	if _, err := os.Stat(tapctl); err != nil {
		// Not the bundled runtime (e.g. an existing OpenVPN install): it manages
		// its own adapters.
		return nil, nil
	}

	adapterMu.Lock()
	defer adapterMu.Unlock()

	if devType == "tap" {
		if err := ensureAdapters(tapctl, hwidTAP, adapterNameTAP, inUse+1); err != nil {
			return nil, err
		}
		return []string{"--disable-dco"}, nil
	}
	if err := ensureAdapters(tapctl, hwidDCO, adapterNameDCO, inUse+1); err != nil {
		return nil, err
	}
	// Fallback for profiles DCO can't handle (old ciphers, compression, ...).
	if err := ensureAdapters(tapctl, hwidTAP, adapterNameTAP, 1); err != nil {
		log.Printf("openvpn: TAP fallback adapter unavailable: %v", err)
	}
	return nil, nil
}

// ensureAdapters makes sure at least want adapters with hwid exist.
func ensureAdapters(tapctl, hwid, baseName string, want int) error {
	have, names, err := listAdapters(tapctl, hwid)
	if err != nil {
		return err
	}
	for i := 0; have < want && i < want+8; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s %d", baseName, i+1)
		}
		if names[strings.ToLower(name)] {
			continue
		}
		if out, err := runTapctl(tapctl, "create", "--hwid", hwid, "--name", name); err != nil {
			return fmt.Errorf("create %s adapter: %v: %s", hwid, err, strings.TrimSpace(out))
		}
		log.Printf("openvpn: created network adapter %q (%s)", name, hwid)
		names[strings.ToLower(name)] = true
		have++
	}
	if have < want {
		return fmt.Errorf("no free %s network adapter", hwid)
	}
	return nil
}

// listAdapters parses "tapctl list" output: one "{GUID}\tName" line per adapter.
func listAdapters(tapctl, hwid string) (int, map[string]bool, error) {
	out, err := runTapctl(tapctl, "list", "--hwid", hwid)
	if err != nil {
		return 0, nil, fmt.Errorf("list %s adapters: %v: %s", hwid, err, strings.TrimSpace(out))
	}
	names := map[string]bool{}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		count++
		if _, name, ok := strings.Cut(line, "\t"); ok {
			names[strings.ToLower(strings.TrimSpace(name))] = true
		}
	}
	return count, names, nil
}

func runTapctl(tapctl string, args ...string) (string, error) {
	// Creating the first adapter can take a while (driver start).
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tapctl, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
