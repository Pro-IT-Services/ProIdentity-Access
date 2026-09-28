package daemon

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wg-client/internal/ipc"
)

// OpenVPN output handling: every session's openvpn log is kept in a file
// (for support) and scanned so the app can show *why* a connection fails
// instead of an endless "Connecting".

// connectTimeout ends an attempt that never reaches CONNECTED.
const connectTimeout = 90 * time.Second

// logDir holds the per-connection OpenVPN logs.
func openvpnLogDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return filepath.Join(base, "ProIdentity", "logs")
	}
	return "/Library/Logs/ProIdentity"
}

// diagnosis maps an openvpn log line to a message for the user. fatal means
// OpenVPN won't recover by retrying.
type diagnosis struct {
	message string
	fatal   bool
}

var diagnoses = []struct {
	match []string
	diagnosis
}{
	{[]string{"AUTH_FAILED"}, diagnosis{"The server rejected the username or password.", true}},
	{[]string{"All ovpn-dco adapters", "All TAP-Windows adapters", "There are no TAP-Windows", "Cannot open TUN/TAP dev", "no free adapter"},
		diagnosis{"No free VPN network adapter. Close other VPN apps and try again.", true}},
	{[]string{"VERIFY ERROR", "certificate verify failed", "CRL: cannot read"},
		diagnosis{"The server's certificate couldn't be verified. The profile may be out of date.", true}},
	{[]string{"Cannot resolve host address", "RESOLVE: Cannot resolve"},
		diagnosis{"Can't find the VPN server: its name couldn't be resolved.", false}},
	{[]string{"TLS key negotiation failed", "TLS handshake failed"},
		diagnosis{"No response from the VPN server. Check that it's reachable (address, port, firewall).", false}},
	{[]string{"Connection refused"}, diagnosis{"The VPN server refused the connection.", false}},
}

func diagnose(line string) (diagnosis, bool) {
	if i := strings.Index(line, "Options error:"); i >= 0 {
		return diagnosis{"The profile uses an option this OpenVPN version can't use: " + strings.TrimSpace(line[i+len("Options error:"):]), true}, true
	}
	for _, d := range diagnoses {
		for _, m := range d.match {
			if strings.Contains(line, m) {
				return d.diagnosis, true
			}
		}
	}
	return diagnosis{}, false
}

// openSessionLog creates the per-connection log file (replaced each attempt).
func openSessionLog(id string) io.WriteCloser {
	dir := openvpnLogDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	name := "openvpn-" + safeFileName(id) + ".log"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return nil
	}
	fmt.Fprintf(f, "# ProIdentity Access OpenVPN session %s, started %s\n", id, time.Now().Format(time.RFC3339))
	return f
}

// watchOutput reads openvpn's log, keeps it on disk and reacts to failures.
func (m *OpenVPNManager) watchOutput(sess *ovpnSession, r io.Reader) {
	logFile := openSessionLog(sess.id)
	if logFile != nil {
		defer logFile.Close()
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if logFile != nil {
			io.WriteString(logFile, line+"\n")
		}
		if strings.Contains(line, "ERROR") || strings.Contains(line, "error") || strings.Contains(line, "failed") {
			sess.mu.Lock()
			sess.lastLogError = trimLogPrefix(line)
			sess.mu.Unlock()
		}
		// Remember where it's trying to connect, so a timeout can name it.
		if addr, proto := remoteAttempt(line); addr != "" {
			sess.mu.Lock()
			sess.hint = fmt.Sprintf("Can't reach the VPN server at %s (%s). Check that the server is running and that its firewall allows this port.", addr, proto)
			sess.mu.Unlock()
		}
		d, ok := diagnose(line)
		if !ok {
			continue
		}
		sess.mu.Lock()
		connected := sess.status == ipc.StatusConnected
		sess.mu.Unlock()
		if connected {
			continue
		}
		if d.fatal {
			log.Printf("openvpn %s: %s (%s)", sess.id, d.message, trimLogPrefix(line))
			sess.setError(d.message)
			m.emit(sess)
			go m.stop(sess)
			continue
		}
		// Retrying on its own; tell the user what it's waiting on (keep the
		// more specific "can't reach <address>" hint if there is one).
		sess.mu.Lock()
		if !strings.HasPrefix(sess.hint, "Can't reach the VPN server at") {
			sess.hint = d.message
		}
		sess.mu.Unlock()
	}
}

// watchConnectTimeout ends an attempt that doesn't connect in time.
func (m *OpenVPNManager) watchConnectTimeout(sess *ovpnSession) {
	// Early notice: say what it's waiting on while it keeps trying.
	time.Sleep(20 * time.Second)
	sess.mu.Lock()
	early := !sess.stopped && sess.status == ipc.StatusConnecting && sess.hint != ""
	if early {
		sess.errMsg = sess.hint
	}
	sess.mu.Unlock()
	if early {
		m.emit(sess)
	}

	time.Sleep(connectTimeout - 20*time.Second)
	sess.mu.Lock()
	stuck := !sess.stopped && sess.status == ipc.StatusConnecting
	msg := sess.hint
	if msg == "" && sess.lastLogError != "" {
		msg = "OpenVPN reported: " + sess.lastLogError
	}
	sess.mu.Unlock()
	if !stuck {
		return
	}
	if msg == "" {
		msg = "Couldn't connect within 90 seconds."
	}
	log.Printf("openvpn %s: gave up after %s: %s", sess.id, connectTimeout, msg)
	sess.setError(msg)
	m.emit(sess)
	m.stop(sess)
}

// remoteAttempt extracts the server address from openvpn's connect lines:
//
//	Attempting to establish TCP connection with [AF_INET]203.0.113.10:1194
//	UDP link remote: [AF_INET]203.0.113.10:1194
func remoteAttempt(line string) (addr, proto string) {
	for _, p := range []struct{ marker, proto string }{
		{"Attempting to establish TCP connection with ", "TCP"},
		{"UDP link remote: ", "UDP"},
	} {
		if i := strings.Index(line, p.marker); i >= 0 {
			a := strings.TrimSpace(line[i+len(p.marker):])
			if j := strings.Index(a, "]"); j >= 0 {
				a = a[j+1:]
			}
			return a, p.proto
		}
	}
	return "", ""
}

// trimLogPrefix drops openvpn's timestamp ("2026-09-28 19:05:01 ").
func trimLogPrefix(line string) string {
	if len(line) > 20 && line[4] == '-' && line[10] == ' ' && line[13] == ':' {
		return strings.TrimSpace(line[20:])
	}
	return strings.TrimSpace(line)
}

func safeFileName(id string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
}
