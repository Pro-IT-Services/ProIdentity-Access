package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wg-client/internal/ipc"
)

// TestOpenVPNManagementEndToEnd drives the installed openvpn through the real
// management code and checks it gets past authentication to the connect step.
// Opt-in (needs an installed OpenVPN runtime, no admin rights):
//
//	PI_OPENVPN_E2E=1 go test ./internal/daemon -run OpenVPNManagementEndToEnd -v
func TestOpenVPNManagementEndToEnd(t *testing.T) {
	if os.Getenv("PI_OPENVPN_E2E") == "" {
		t.Skip("set PI_OPENVPN_E2E=1 to run against the installed openvpn")
	}
	exe := filepath.Join(os.Getenv("ProgramFiles"), "ProIdentity Access", "openvpn", "bin", "openvpn.exe")
	if runtime.GOOS != "windows" {
		exe = "/Library/ProIdentity/openvpn/openvpn"
	}
	if _, err := os.Stat(exe); err != nil {
		t.Skip("openvpn runtime not installed")
	}

	for i := 0; i < 5; i++ {
		dir := t.TempDir()
		fp := strings.TrimSuffix(strings.Repeat("AB:", 32), ":")
		cfgPath := filepath.Join(dir, "t.ovpn")
		os.WriteFile(cfgPath, []byte("client\ndev tun\nproto udp\nremote 127.0.0.1 1\nnobind\npeer-fingerprint "+fp+"\nauth-user-pass\n"), 0o600)
		mgmtPw, pwPath, err := writeManagementPassword(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		port, _ := freeLocalPort()
		logPath := filepath.Join(dir, "openvpn.log")
		out, _ := os.Create(logPath)
		cmd := exec.Command(exe, "--config", cfgPath, "--management", "127.0.0.1", fmt.Sprint(port), pwPath,
			"--management-hold", "--management-query-passwords", "--auth-nocache", "--verb", "3")
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		m := NewOpenVPNManager(nil)
		sess := &ovpnSession{id: "e2e", status: ipc.StatusConnecting, cmd: cmd,
			username: `mpan`, password: `Se"cret\pass 123!`, mgmtPassword: mgmtPw}
		go m.drive(sess, port)

		deadline := time.Now().Add(8 * time.Second)
		reached := false
		for time.Now().Before(deadline) {
			data, _ := os.ReadFile(logPath)
			if strings.Contains(string(data), "UDP link remote") {
				reached = true
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		m.stop(sess)
		cmd.Wait()
		out.Close()
		data, _ := os.ReadFile(logPath)
		log := string(data)
		if !reached || !strings.Contains(log, "CMD 'password [...]'") {
			t.Fatalf("attempt %d did not get past authentication:\n%s", i, log)
		}
	}
}
