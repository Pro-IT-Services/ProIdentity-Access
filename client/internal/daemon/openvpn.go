package daemon

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// known remembers each profile's networks from its last connection, so an
	// overlapping profile is refused before it even starts next time.
	known map[string]netSet
	// conflict reports a live connection already using any of n (set by the
	// TunnelManager, which also sees WireGuard tunnels).
	conflict func(except, ownerID string, n netSet) (string, bool)
}

func NewOpenVPNManager(broadcast func(ipc.Event)) *OpenVPNManager {
	return &OpenVPNManager{sessions: make(map[string]*ovpnSession), broadcast: broadcast, known: map[string]netSet{}}
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
	mgmt    *mgmtChannel
	cfgPath string

	username string
	password string
	stopped  bool

	mgmtPassword string // per-session management password
	mgmtPwPath   string
	profileNets  netSet         // networks written in the profile
	networks     netSet         // profile + pushed by the server
	remote       string         // server address being used
	connectedAt  time.Time      // when the tunnel came up
	errFinal     bool           // an error that later messages must not replace
	logPipe      *io.PipeWriter // openvpn stdout/stderr → watchOutput
	lastLogError string         // last error-looking log line
	hint         string         // what a still-retrying attempt is waiting on
}

func (s *ovpnSession) snapshot() ipc.OpenVPNStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ipc.OpenVPNStatus{
		ID: s.id, OwnerID: s.ownerID, Name: s.name, Status: s.status,
		DevType: s.devType, IP: s.ip, RxBytes: s.rx, TxBytes: s.tx, Error: s.errMsg,
		Remote: s.remote, Networks: s.networks.prefixStrings(), ConnectedAt: unixOrZero(s.connectedAt),
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

	// Refuse right away if this profile's networks (from the profile, or learnt
	// on a previous connection) are already used by another live VPN.
	profileNets := profileNetworks(cfg)
	m.mu.Lock()
	want := profileNets.merge(m.known[paramsID(p)])
	m.mu.Unlock()
	if msg, bad := m.checkConflict(paramsID(p), ownerID, want); bad {
		return nil, errors.New(msg)
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

	bin, ok := openvpnBinaryPath()
	if !ok {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("the OpenVPN component is missing from this installation; reinstall ProIdentity Access")
	}
	mgmtPassword, mgmtPwPath, err := writeManagementPassword(cfgPath)
	if err != nil {
		_ = os.Remove(cfgPath)
		return nil, fmt.Errorf("management password: %w", err)
	}
	args := []string{
		"--config", cfgPath,
		// The management port is on localhost; a per-session password (file
		// readable only by this service) keeps other local users out of it.
		"--management", "127.0.0.1", strconv.Itoa(port), mgmtPwPath,
		"--management-hold",
		"--management-query-passwords",
		"--auth-nocache",
		"--verb", "3",
	}
	extra, err := prepareOpenVPNAdapter(bin, devType, m.activeOfType(devType, paramsID(p)))
	if err != nil {
		_ = os.Remove(cfgPath)
		return nil, err
	}
	args = append(args, extra...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.Dir(cfgPath)
	logR, logW := io.Pipe()
	cmd.Stdout = logW
	cmd.Stderr = logW

	sess := &ovpnSession{
		id: paramsID(p), ownerID: ownerID, name: p.Name, devType: devType,
		status: ipc.StatusConnecting, cmd: cmd, cfgPath: cfgPath,
		username: p.Username, password: p.Password,
		logPipe: logW, mgmtPassword: mgmtPassword, mgmtPwPath: mgmtPwPath,
		profileNets: profileNets, networks: profileNets,
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(cfgPath)
		_ = os.Remove(mgmtPwPath)
		return nil, fmt.Errorf("start openvpn (%s): %w", bin, err)
	}

	m.mu.Lock()
	m.sessions[sess.id] = sess
	m.mu.Unlock()
	m.emit(sess)

	go m.watchOutput(sess, logR)
	go m.reap(sess)
	go m.drive(sess, port)
	go m.watchConnectTimeout(sess)

	st := sess.snapshot()
	return &st, nil
}

// activeOfType counts running sessions of devType other than id (each needs
// its own network adapter on Windows).
func (m *OpenVPNManager) activeOfType(devType, id string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for sid, s := range m.sessions {
		s.mu.Lock()
		if sid != id && s.devType == devType && !s.stopped {
			n++
		}
		s.mu.Unlock()
	}
	return n
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
		mgmt.write("signal SIGTERM")
		time.Sleep(300 * time.Millisecond)
		mgmt.close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// reap waits for the process to exit and finalizes the session.
func (m *OpenVPNManager) reap(sess *ovpnSession) {
	_ = sess.cmd.Wait()
	if sess.logPipe != nil {
		_ = sess.logPipe.Close()
	}

	sess.mu.Lock()
	if sess.status != ipc.StatusError {
		sess.status = ipc.StatusDisconnected
	}
	cfgPath := sess.cfgPath
	pwPath := sess.mgmtPwPath
	sess.mu.Unlock()
	if pwPath != "" {
		_ = os.Remove(pwPath)
	}

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
	ch := newMgmtChannel(conn)
	defer ch.close()
	sess.mu.Lock()
	sess.mgmt = ch
	sess.mu.Unlock()

	// One command at a time (see mgmtChannel): authenticate (openvpn prompts
	// "ENTER PASSWORD:"), enable notifications, release the start hold.
	ch.send(sess.mgmtPassword)
	ch.send("state on")
	ch.send("bytecount 5")
	ch.send("hold release")

	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line, isReply := ch.reply(strings.TrimRight(sc.Text(), "\r"))
		if isReply || line == "" {
			continue
		}
		m.handleMgmtLine(sess, ch, line)
	}
}

func (m *OpenVPNManager) handleMgmtLine(sess *ovpnSession, ch *mgmtChannel, line string) {
	switch {
	case strings.HasPrefix(line, ">PASSWORD:Need 'Auth'"):
		ch.send(`username "Auth" ` + mgmtEscape(sess.username))
		ch.send(`password "Auth" ` + mgmtEscape(sess.password))

	case strings.HasPrefix(line, ">PASSWORD:Verification Failed"):
		sess.setError("The server rejected the username or password.")
		m.emit(sess)
		m.stop(sess)

	// Prompts the app can't answer: say so instead of waiting forever.
	case strings.HasPrefix(line, ">PASSWORD:Need 'Private Key'"):
		sess.setError("This profile's private key is protected by a passphrase, which isn't supported. Ask your administrator for a profile without one.")
		m.emit(sess)
		go m.stop(sess)
	case strings.HasPrefix(line, ">PASSWORD:Need 'HTTP Proxy'"):
		sess.setError("This profile needs an HTTP proxy password, which isn't supported.")
		m.emit(sess)
		go m.stop(sess)
	case strings.HasPrefix(line, ">NEED-OK:"):
		// e.g. a confirmation OpenVPN would ask a person; accept it.
		if name := quoted(line); name != "" {
			ch.send("needok " + name + " ok")
		}
	case strings.HasPrefix(line, ">FATAL:"):
		sess.setError("OpenVPN stopped: " + strings.TrimPrefix(line, ">FATAL:"))
		m.emit(sess)

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
		sess.errMsg = "" // clear an earlier "still trying" notice
		if sess.connectedAt.IsZero() {
			sess.connectedAt = time.Now()
		}
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

// writeManagementPassword stores a random management password next to the
// profile (same service-only temp folder, mode 0600).
func writeManagementPassword(cfgPath string) (string, string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	pw := hex.EncodeToString(buf)
	path := strings.TrimSuffix(cfgPath, filepath.Ext(cfgPath)) + ".mgmt"
	if err := os.WriteFile(path, []byte(pw+"\n"), 0o600); err != nil {
		return "", "", err
	}
	return pw, path, nil
}

// setFinalError sets an error that later messages (e.g. openvpn's own
// "NETSH: command failed" as it shuts down) must not replace.
func (s *ovpnSession) setFinalError(msg string) {
	s.mu.Lock()
	s.status = ipc.StatusError
	s.errMsg = msg
	s.errFinal = true
	s.mu.Unlock()
}

func (m *OpenVPNManager) checkConflict(id, ownerID string, n netSet) (string, bool) {
	if m.conflict == nil || n.empty() {
		return "", false
	}
	return m.conflict(id, ownerID, n)
}

// liveNet is one running connection's claim, for conflict checks.
type liveNet struct {
	id, name, ownerID string
	nets              netSet
}

// liveNetworks lists connecting/connected sessions other than except.
func (m *OpenVPNManager) liveNetworks(except string) []liveNet {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []liveNet
	for id, s := range m.sessions {
		if id == except {
			continue
		}
		s.mu.Lock()
		live := !s.stopped && (s.status == ipc.StatusConnecting || s.status == ipc.StatusConnected)
		if live && !s.networks.empty() {
			out = append(out, liveNet{id: id, name: s.name, ownerID: s.ownerID, nets: s.networks})
		}
		s.mu.Unlock()
	}
	return out
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// quoted returns the first '...' quoted word of a management line.
func quoted(line string) string {
	i := strings.IndexByte(line, '\'')
	if i < 0 {
		return ""
	}
	j := strings.IndexByte(line[i+1:], '\'')
	if j < 0 {
		return ""
	}
	return line[i+1 : i+1+j]
}

func (s *ovpnSession) setError(msg string) {
	s.mu.Lock()
	if s.errFinal {
		s.mu.Unlock()
		return
	}
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

// openvpnBinaryPath finds the OpenVPN runtime shipped with this installation:
// Windows <install>\openvpn\bin\openvpn.exe, macOS /Library/ProIdentity/openvpn/openvpn.
//
// This service runs as SYSTEM / root, so it only runs a binary from a location
// ordinary users can't write to: never PATH, Homebrew or /usr/local.
func openvpnBinaryPath() (string, bool) {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(dir, "openvpn", "bin", "openvpn.exe"))
		} else {
			candidates = append(candidates, filepath.Join(dir, "openvpn", "openvpn"))
		}
	}
	if runtime.GOOS == "linux" {
		candidates = append(candidates, "/usr/sbin/openvpn", "/usr/bin/openvpn")
	}
	for _, cand := range candidates {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, true
		}
	}
	return "", false
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
