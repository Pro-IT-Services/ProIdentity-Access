package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"wg-client/internal/ipc"
)

// OpenVPNManager runs and supervises OpenVPN sessions. Each session is a
// bundled `openvpn` process driven over its management interface: we release
// the start hold, answer credential prompts, watch state transitions, and read
// byte counters. Sessions are isolated per OS user (ownerID), mirroring the
// WireGuard TunnelManager.
type OpenVPNManager struct {
	mu        sync.Mutex
	sessions  map[string]*ovpnSession
	broadcast func(ipc.Event)
}

func NewOpenVPNManager(broadcast func(ipc.Event)) *OpenVPNManager {
	return &OpenVPNManager{sessions: make(map[string]*ovpnSession), broadcast: broadcast}
}

type ovpnSession struct {
	mu      sync.Mutex
	id      string
	ownerID string
	name    string
	devType string
	status  ipc.TunnelStatus
	ip      string
	rx, tx  int64
	errMsg  string

	cmd     *exec.Cmd
	mgmt    net.Conn
	cfgPath string

	username string
	password string
	stopped  bool
}

func (s *ovpnSession) snapshot() ipc.OpenVPNStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ipc.OpenVPNStatus{
		ID: s.id, OwnerID: s.ownerID, Name: s.name, Status: s.status,
		DevType: s.devType, IP: s.ip, RxBytes: s.rx, TxBytes: s.tx, Error: s.errMsg,
	}
}

func (m *OpenVPNManager) emit(s *ovpnSession) {
	if m.broadcast == nil {
		return
	}
	m.broadcast(mustEvent(ipc.EventOpenVPNChanged, s.snapshot()))
}

// ConnectOpenVPN launches (or relaunches) an OpenVPN session for ownerID.
func (m *OpenVPNManager) ConnectOpenVPN(ownerID string, p ipc.OpenVPNConnectParams) (*ipc.OpenVPNStatus, error) {
	if strings.TrimSpace(p.Config) == "" {
		return nil, fmt.Errorf("empty openvpn config")
	}
	// Replace any existing session with the same id for this owner.
	m.mu.Lock()
	if existing, ok := m.sessions[paramsID(p)]; ok && existing.ownerID == ownerID {
		m.mu.Unlock()
		m.stop(existing)
		m.mu.Lock()
	}
	m.mu.Unlock()

	devType := "tun"
	if strings.ToLower(p.DevType) == "tap" {
		devType = "tap"
	}

	cfg := p.Config
	// For a TAP profile with a user-chosen address, pin it via ifconfig so the
	// adapter comes up statically (cross-platform; avoids OS-specific tooling).
	if devType == "tap" && strings.TrimSpace(p.CustomIP) != "" {
		cfg = strings.TrimRight(cfg, "\n") + "\nifconfig " + strings.TrimSpace(p.CustomIP) + " 255.255.255.0\n"
	}

	cfgPath, err := writeTempConfig(paramsID(p), cfg)
	if err != nil {
		return nil, err
	}

	port, err := freeLocalPort()
	if err != nil {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("allocate management port: %w", err)
	}

	bin := openvpnBinaryPath()
	args := []string{
		"--config", cfgPath,
		"--management", "127.0.0.1", strconv.Itoa(port),
		"--management-hold",
		"--management-query-passwords",
		"--auth-nocache",
		"--verb", "3",
	}
	// On Windows, drive TUN through Wintun so a plain "dev tun" profile connects
	// without the TAP-Windows driver installed. TAP profiles still require it.
	if runtime.GOOS == "windows" && devType == "tun" {
		args = append(args, "--windows-driver", "wintun")
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.Dir(cfgPath)

	sess := &ovpnSession{
		id: paramsID(p), ownerID: ownerID, name: p.Name, devType: devType,
		status: ipc.StatusConnecting, cmd: cmd, cfgPath: cfgPath,
		username: p.Username, password: p.Password,
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("start openvpn (%s): %w", bin, err)
	}

	m.mu.Lock()
	m.sessions[sess.id] = sess
	m.mu.Unlock()
	m.emit(sess)

	go m.reap(sess)
	go m.drive(sess, port)

	st := sess.snapshot()
	return &st, nil
}

// DisconnectOpenVPN stops a session the caller owns.
func (m *OpenVPNManager) DisconnectOpenVPN(ownerID, id string) error {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()
	if sess == nil || sess.ownerID != ownerID {
		return fmt.Errorf("session not found")
	}
	m.stop(sess)
	return nil
}

// ListOpenVPN returns sessions owned by ownerID.
func (m *OpenVPNManager) ListOpenVPN(ownerID string) []ipc.OpenVPNStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []ipc.OpenVPNStatus{}
	for _, s := range m.sessions {
		if s.ownerID == ownerID {
			out = append(out, s.snapshot())
		}
	}
	return out
}

// StopAll tears down every running session (daemon shutdown).
func (m *OpenVPNManager) StopAll() {
	m.mu.Lock()
	all := make([]*ovpnSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		m.stop(s)
	}
}

func (m *OpenVPNManager) stop(sess *ovpnSession) {
	sess.mu.Lock()
	if sess.stopped {
		sess.mu.Unlock()
		return
	}
	sess.stopped = true
	mgmt := sess.mgmt
	cmd := sess.cmd
	sess.mu.Unlock()

	if mgmt != nil {
		_, _ = mgmt.Write([]byte("signal SIGTERM\n"))
		time.Sleep(300 * time.Millisecond)
		_ = mgmt.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// reap waits for the process to exit and finalizes the session.
func (m *OpenVPNManager) reap(sess *ovpnSession) {
	_ = sess.cmd.Wait()

	sess.mu.Lock()
	if sess.status != ipc.StatusError {
		sess.status = ipc.StatusDisconnected
	}
	cfgPath := sess.cfgPath
	sess.mu.Unlock()

	if cfgPath != "" {
		_ = os.Remove(cfgPath) // remove the profile (embedded keys) from disk
	}
	m.emit(sess)

	m.mu.Lock()
	if cur, ok := m.sessions[sess.id]; ok && cur == sess {
		delete(m.sessions, sess.id)
	}
	m.mu.Unlock()
}

// drive connects to the management interface and runs the control loop.
func (m *OpenVPNManager) drive(sess *ovpnSession, port int) {
	conn, err := dialManagement(port, 5*time.Second)
	if err != nil {
		sess.setError("management connect failed: " + err.Error())
		m.emit(sess)
		m.stop(sess)
		return
	}
	sess.mu.Lock()
	sess.mgmt = conn
	sess.mu.Unlock()

	// Enable notifications and release the start hold.
	_, _ = conn.Write([]byte("state on\n"))
	_, _ = conn.Write([]byte("bytecount 5\n"))
	_, _ = conn.Write([]byte("hold release\n"))

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		m.handleMgmtLine(sess, conn, line)
	}
}

func (m *OpenVPNManager) handleMgmtLine(sess *ovpnSession, conn net.Conn, line string) {
	switch {
	case strings.HasPrefix(line, ">PASSWORD:Need 'Auth'"):
		_, _ = conn.Write([]byte("username \"Auth\" " + mgmtEscape(sess.username) + "\n"))
		_, _ = conn.Write([]byte("password \"Auth\" " + mgmtEscape(sess.password) + "\n"))

	case strings.HasPrefix(line, ">PASSWORD:Verification Failed"):
		sess.setError("authentication failed")
		m.emit(sess)
		m.stop(sess)

	case strings.HasPrefix(line, ">STATE:"):
		m.handleState(sess, strings.TrimPrefix(line, ">STATE:"))

	case strings.HasPrefix(line, ">BYTECOUNT:"):
		parts := strings.Split(strings.TrimPrefix(line, ">BYTECOUNT:"), ",")
		if len(parts) == 2 {
			rx, _ := strconv.ParseInt(parts[0], 10, 64)
			tx, _ := strconv.ParseInt(parts[1], 10, 64)
			sess.mu.Lock()
			sess.rx, sess.tx = rx, tx
			sess.mu.Unlock()
			m.emit(sess)
		}
	}
}

// handleState parses a management STATE line:
//
//	<time>,<state>,<description>,<local-ip>,<remote-ip>,...
func (m *OpenVPNManager) handleState(sess *ovpnSession, payload string) {
	f := strings.Split(payload, ",")
	if len(f) < 2 {
		return
	}
	state := f[1]
	changed := false
	sess.mu.Lock()
	switch state {
	case "CONNECTED":
		sess.status = ipc.StatusConnected
		if len(f) >= 4 && f[3] != "" {
			sess.ip = f[3]
		}
		changed = true
	case "RECONNECTING", "WAIT", "AUTH", "GET_CONFIG", "ASSIGN_IP", "RESOLVE", "TCP_CONNECT":
		if sess.status != ipc.StatusConnected {
			sess.status = ipc.StatusConnecting
			changed = true
		}
	case "EXITING":
		if sess.status != ipc.StatusError {
			sess.status = ipc.StatusDisconnected
			changed = true
		}
	}
	sess.mu.Unlock()
	if changed {
		m.emit(sess)
	}
}

func (s *ovpnSession) setError(msg string) {
	s.mu.Lock()
	s.status = ipc.StatusError
	s.errMsg = msg
	s.mu.Unlock()
}

// --- helpers ---

func paramsID(p ipc.OpenVPNConnectParams) string {
	if strings.TrimSpace(p.ID) != "" {
		return p.ID
	}
	return "ovpn-" + p.Name
}

// mgmtEscape quotes a value for the OpenVPN management protocol: backslashes and
// double quotes are backslash-escaped and the whole value is wrapped in quotes.
func mgmtEscape(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}

func dialManagement(port int, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			return conn, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out dialing %s", addr)
}

func freeLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// openvpnBinaryPath finds the openvpn binary: bundled next to the daemon, then
// a standard install location, then PATH. This lets the client drive an
// OpenVPN Community install with no extra configuration.
func openvpnBinaryPath() string {
	name := "openvpn"
	if runtime.GOOS == "windows" {
		name = "openvpn.exe"
	}

	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, name), filepath.Join(dir, "openvpn", name))
	}
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\OpenVPN\bin\openvpn.exe`,
			`C:\Program Files (x86)\OpenVPN\bin\openvpn.exe`,
		)
	case "darwin":
		candidates = append(candidates,
			"/opt/homebrew/sbin/openvpn",
			"/usr/local/sbin/openvpn",
			"/usr/local/opt/openvpn/sbin/openvpn",
		)
	default: // linux
		candidates = append(candidates, "/usr/sbin/openvpn", "/usr/bin/openvpn")
	}

	for _, cand := range candidates {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return name // last resort: rely on PATH
}

// writeTempConfig writes the profile to a per-session file with tight perms.
func writeTempConfig(id, cfg string) (string, error) {
	dir := filepath.Join(os.TempDir(), "proidentity-ovpn")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
	path := filepath.Join(dir, safe+".ovpn")
	if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
		return "", err
	}
	return path, nil
}

func mustEvent(typ string, payload any) ipc.Event {
	data, _ := json.Marshal(payload)
	return ipc.Event{Type: typ, Payload: data}
}
