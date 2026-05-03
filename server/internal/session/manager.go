package session

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/firewall"
	"proidentity/internal/model"
	"proidentity/internal/wireguard"
)

// Manager handles the full session lifecycle: create, keepalive, teardown.
type Manager struct {
	db       *sqlx.DB
	registry *wireguard.Registry
	fw       *firewall.Manager
	settings func(key string) string // live settings lookup
}

func NewManager(db *sqlx.DB, registry *wireguard.Registry, fw *firewall.Manager, settings func(string) string) *Manager {
	return &Manager{db: db, registry: registry, fw: fw, settings: settings}
}

// CreateSession allocates an IP on the given server, adds a WG peer, installs firewall rules,
// and returns the full WireGuard client config string.
func (m *Manager) CreateSession(userID, serverID, clientPubKey string) (*model.Session, string, error) {
	inst, err := m.registry.Get(serverID)
	if err != nil {
		return nil, "", fmt.Errorf("server not available: %w", err)
	}

	sessionID := uuid.New().String()

	// Allocate IP from the server's pool
	assignedIP, err := AllocateIP(m.db, serverID, sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("allocate ip: %w", err)
	}

	// Insert session record
	sess := &model.Session{
		ID:              sessionID,
		UserID:          userID,
		ServerID:        &serverID,
		ClientPublicKey: clientPubKey,
		AssignedIP:      assignedIP,
		CreatedAt:       time.Now(),
		LastKeepalive:   time.Now(),
	}
	_, err = m.db.Exec(`
		INSERT INTO sessions (id, user_id, server_id, client_public_key, assigned_ip, created_at, last_keepalive)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		sess.ID, sess.UserID, sess.ServerID, sess.ClientPublicKey, sess.AssignedIP,
	)
	if err != nil {
		_ = ReleaseIP(m.db, serverID, assignedIP)
		return nil, "", fmt.Errorf("insert session: %w", err)
	}

	// Add WireGuard peer then do a full config sync
	if err := inst.Manager.AddPeer(clientPubKey, assignedIP); err != nil {
		_ = m.deleteSession(sess)
		return nil, "", fmt.Errorf("wg add peer: %w", err)
	}
	m.syncServerPeers(serverID)

	resources, err := m.userResources(serverID, userID)
	if err != nil {
		log.Printf("warn: get resources for server %s: %v", serverID, err)
	}
	if len(resources) > 0 {
		if err := m.fw.AddRules(assignedIP, resources); err != nil {
			log.Printf("warn: add firewall rules for %s: %v", assignedIP, err)
		}
	}

	// Build WireGuard config from server info
	srv := inst.Server
	serverPubKey := srv.PublicKey
	endpoint := fmt.Sprintf("%s:%d", srv.Endpoint, srv.Port)

	dns := ""
	if srv.DNS != nil && *srv.DNS != "" {
		dns = *srv.DNS
	}

	allowedIPs := buildAllowedIPs(resources, assignedIP, srv.Subnet)

	dnsLine := ""
	if dns != "" {
		dnsLine = "DNS = " + dns + "\n"
	}

	cfg := fmt.Sprintf(`[Interface]
Address = %s/32
%s
[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, assignedIP, dnsLine, serverPubKey, endpoint, allowedIPs)

	return sess, cfg, nil
}

// Keepalive updates the last_keepalive timestamp for a session.
func (m *Manager) Keepalive(sessionID string) error {
	res, err := m.db.Exec("UPDATE sessions SET last_keepalive=NOW() WHERE id=?", sessionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// Terminate explicitly tears down a session.
func (m *Manager) Terminate(sessionID string) error {
	var sess model.Session
	if err := m.db.Get(&sess, "SELECT * FROM sessions WHERE id=?", sessionID); err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	return m.deleteSession(&sess)
}

// TerminateUserSessions tears down all sessions for a user.
func (m *Manager) TerminateUserSessions(userID string) error {
	var sessions []model.Session
	if err := m.db.Select(&sessions, "SELECT * FROM sessions WHERE user_id=?", userID); err != nil {
		return err
	}
	for _, s := range sessions {
		s := s
		if err := m.deleteSession(&s); err != nil {
			log.Printf("warn: terminate session %s: %v", s.ID, err)
		}
	}
	return nil
}

// TerminateAll tears down every active session. Called on daemon shutdown.
func (m *Manager) TerminateAll() {
	var sessions []model.Session
	if err := m.db.Select(&sessions, "SELECT * FROM sessions"); err != nil {
		log.Printf("warn: terminate all: %v", err)
		return
	}
	for _, s := range sessions {
		s := s
		if err := m.deleteSession(&s); err != nil {
			log.Printf("warn: terminate session %s: %v", s.ID, err)
		}
	}
}

// StartWatchdog runs background goroutines for session expiry and dead-peer cleanup.
func (m *Manager) StartWatchdog() {
	// Expire sessions that stopped sending keepalives
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.expireStale()
		}
	}()

	// Remove iptables rules and terminate sessions whose WireGuard tunnel is dead
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			m.cleanDeadPeers()
		}
	}()
}

func (m *Manager) expireStale() {
	timeoutSec, _ := strconv.Atoi(m.settings("session_timeout"))
	if timeoutSec <= 0 {
		timeoutSec = 90
	}

	var stale []model.Session
	err := m.db.Select(&stale, `
		SELECT * FROM sessions
		WHERE last_keepalive < DATE_SUB(NOW(), INTERVAL ? SECOND)`,
		timeoutSec,
	)
	if err != nil || len(stale) == 0 {
		return
	}

	for _, s := range stale {
		s := s
		log.Printf("session %s expired (user %s, ip %s)", s.ID, s.UserID, s.AssignedIP)
		if err := m.deleteSession(&s); err != nil {
			log.Printf("warn: expire session %s: %v", s.ID, err)
		}
	}
}

// cleanDeadPeers checks every active session's WireGuard handshake age.
// Sessions where the tunnel has been silent for more than deadThreshold are torn down.
const deadThreshold = 3 * time.Minute

func (m *Manager) cleanDeadPeers() {
	var sessions []model.Session
	if err := m.db.Select(&sessions, "SELECT * FROM sessions"); err != nil {
		return
	}

	for _, s := range sessions {
		s := s
		if s.ServerID == nil {
			continue
		}
		// Skip sessions that are too young — give them time to complete the first handshake
		if time.Since(s.CreatedAt) < deadThreshold {
			continue
		}

		inst, err := m.registry.Get(*s.ServerID)
		if err != nil {
			continue
		}

		age, err := inst.Manager.PeerHandshakeAge(s.ClientPublicKey)
		if err != nil {
			// Peer is not present on the WireGuard interface at all — clean up
			log.Printf("peer %s not on interface, cleaning session %s", s.ClientPublicKey[:8], s.ID)
			if err := m.deleteSession(&s); err != nil {
				log.Printf("warn: cleanup ghost session %s: %v", s.ID, err)
			}
			continue
		}

		// age == 0 means peer exists but has never completed a handshake
		// age > deadThreshold means the tunnel has gone silent
		if age == 0 || age > deadThreshold {
			log.Printf("dead peer %s (handshake age %v), terminating session %s",
				s.ClientPublicKey[:8], age, s.ID)
			if err := m.deleteSession(&s); err != nil {
				log.Printf("warn: terminate dead session %s: %v", s.ID, err)
			}
		}
	}
}

func (m *Manager) deleteSession(sess *model.Session) error {
	serverID := ""
	if sess.ServerID != nil {
		serverID = *sess.ServerID
	}

	// Remove WG peer from the appropriate server
	if serverID != "" {
		if inst, err := m.registry.Get(serverID); err == nil {
			if err := inst.Manager.RemovePeer(sess.ClientPublicKey); err != nil {
				log.Printf("warn: remove peer %s: %v", sess.ClientPublicKey, err)
			}
		}
	}

	// Remove firewall rules — derived from user+server, matching session creation.
	var resources []firewall.Rule
	if serverID != "" {
		resources, _ = m.userResources(serverID, sess.UserID)
	}
	if len(resources) > 0 {
		if err := m.fw.RemoveRules(sess.AssignedIP, resources); err != nil {
			log.Printf("warn: remove fw rules %s: %v", sess.AssignedIP, err)
		}
	}

	// Release IP back to the server pool
	if sess.ServerID != nil {
		_ = ReleaseIP(m.db, *sess.ServerID, sess.AssignedIP)
	}

	// Delete DB record
	_, err := m.db.Exec("DELETE FROM sessions WHERE id=?", sess.ID)
	if err != nil {
		return err
	}

	// Full config sync after removal
	if serverID != "" {
		m.syncServerPeers(serverID)
	}
	return nil
}

// syncServerPeers rebuilds the complete peer list for a server from the DB
// and does an atomic replace on the WireGuard interface.
func (m *Manager) syncServerPeers(serverID string) {
	inst, err := m.registry.Get(serverID)
	if err != nil {
		return
	}
	var sessions []model.Session
	if err := m.db.Select(&sessions, "SELECT * FROM sessions WHERE server_id=?", serverID); err != nil {
		log.Printf("warn: sync peers query server %s: %v", serverID, err)
		return
	}
	peers := make([]wireguard.PeerEntry, 0, len(sessions))
	for _, s := range sessions {
		peers = append(peers, wireguard.PeerEntry{
			PublicKey:  s.ClientPublicKey,
			AssignedIP: s.AssignedIP,
		})
	}
	if err := inst.Manager.SyncAllPeers(peers); err != nil {
		log.Printf("warn: sync peers server %s: %v", serverID, err)
	}
}

// resourceRow is a lightweight DB row for building firewall rules.
type resourceRow struct {
	IPAddress string  `db:"ip_address"`
	Type      string  `db:"type"`
	Mask      *int    `db:"mask"`
	Ports     *string `db:"ports"`
}

// userResources returns firewall.Rule entries for resources this user can reach
// on the given server. Path: user_bundle_access -> bundle -> resource.
func (m *Manager) userResources(serverID, userID string) ([]firewall.Rule, error) {
	var rows []resourceRow
	err := m.db.Select(&rows, `
		SELECT DISTINCT r.ip_address, r.type, r.mask, r.ports
		FROM resources r
		JOIN resource_group_members rgm ON rgm.resource_id = r.id
		JOIN user_bundle_access uba ON uba.bundle_id = rgm.resource_group_id
		WHERE uba.server_id = ? AND uba.user_id = ?`, serverID, userID)
	if err != nil {
		return nil, err
	}
	rules := make([]firewall.Rule, 0, len(rows))
	for _, row := range rows {
		cidr := resourceCIDR(row)
		ports := ""
		if row.Ports != nil {
			ports = *row.Ports
		}
		rules = append(rules, firewall.Rule{CIDR: cidr, Ports: ports})
	}
	return rules, nil
}

// resourceCIDR returns the CIDR string for a resource row.
func resourceCIDR(r resourceRow) string {
	if r.Type == "network" && r.Mask != nil {
		return fmt.Sprintf("%s/%d", r.IPAddress, *r.Mask)
	}
	return r.IPAddress + "/32"
}

// Setting exposes a single settings value to callers (e.g. the API layer).
func (m *Manager) Setting(key string) string { return m.settings(key) }

// GetSession returns a single session by ID.
func (m *Manager) GetSession(sessionID string) (*model.Session, error) {
	var sess model.Session
	if err := m.db.Get(&sess, "SELECT * FROM sessions WHERE id=?", sessionID); err != nil {
		return nil, err
	}
	return &sess, nil
}

// ListSessions returns all active sessions.
func (m *Manager) ListSessions() ([]model.Session, error) {
	var sessions []model.Session
	err := m.db.Select(&sessions, "SELECT * FROM sessions ORDER BY created_at DESC")
	return sessions, err
}

// buildAllowedIPs returns a comma-separated list of CIDRs the client should route through VPN.
// vpnSubnet is the server's WireGuard subnet (e.g. "10.99.1.0/24") — always included so the
// client can reach the gateway and other peers, otherwise the gateway IP falls through to the
// default route on the client OS and pings get answered by the home router instead of the VPN.
func buildAllowedIPs(rules []firewall.Rule, assignedIP, vpnSubnet string) string {
	if len(rules) == 0 {
		return vpnSubnet
	}
	result := vpnSubnet
	seen := map[string]bool{vpnSubnet: true}
	for _, r := range rules {
		if !seen[r.CIDR] {
			result += ", " + r.CIDR
			seen[r.CIDR] = true
		}
	}
	return result
}
