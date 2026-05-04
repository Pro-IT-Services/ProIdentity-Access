package wireguard

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/firewall"
	"proidentity/internal/ippool"
	"proidentity/internal/model"
)

// ServerInstance holds a running WireGuard server.
type ServerInstance struct {
	Server  model.WGServer
	Manager *Manager
}

// Registry manages multiple WireGuard server instances.
type Registry struct {
	db *sqlx.DB
	fw *firewall.Manager

	mu      sync.RWMutex
	servers map[string]*ServerInstance // server_id → instance
}

func NewRegistry(db *sqlx.DB, fw *firewall.Manager) *Registry {
	return &Registry{
		db:      db,
		fw:      fw,
		servers: make(map[string]*ServerInstance),
	}
}

// Start loads all active servers from DB and brings up their interfaces.
func (r *Registry) Start() error {
	var servers []model.WGServer
	if err := r.db.Select(&servers, "SELECT * FROM wg_servers WHERE is_active=1"); err != nil {
		return fmt.Errorf("load servers: %w", err)
	}

	for _, srv := range servers {
		srv := srv
		if err := r.startServer(&srv); err != nil {
			log.Printf("warn: start server %s (%s): %v", srv.Name, srv.InterfaceName, err)
			continue
		}
		log.Printf("server %q ready (iface=%s)", srv.Name, srv.InterfaceName)
	}
	return nil
}

// Get returns the ServerInstance for the given server ID.
func (r *Registry) Get(serverID string) (*ServerInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.servers[serverID]
	if !ok {
		return nil, fmt.Errorf("server %s not running", serverID)
	}
	return inst, nil
}

// All returns all running server instances.
func (r *Registry) All() []*ServerInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ServerInstance, 0, len(r.servers))
	for _, inst := range r.servers {
		list = append(list, inst)
	}
	return list
}

// UserServers returns all servers a user can connect to.
// New direct path: user_server_access only. The legacy role path was migrated
// into user_server_access by migration 010.
func (r *Registry) UserServers(userID string) ([]model.WGServer, error) {
	servers := []model.WGServer{}
	err := r.db.Select(&servers, `
		SELECT s.*
		FROM wg_servers s
		JOIN user_server_access usa ON usa.server_id = s.id
		WHERE usa.user_id = ? AND s.is_active = 1
		ORDER BY s.name`, userID)
	return servers, err
}

// Create creates a new fully-managed WireGuard server.
func (r *Registry) Create(name, endpoint string, port int, iface, subnet, dns string) (*model.WGServer, error) {
	privKey, pubKey, err := GenerateKeypair()
	if err != nil {
		return nil, fmt.Errorf("generate keypair: %w", err)
	}

	var dnsPtr *string
	if dns != "" {
		dnsPtr = &dns
	}

	srv := model.WGServer{
		ID:            uuid.New().String(),
		Name:          name,
		Endpoint:      endpoint,
		Port:          port,
		InterfaceName: iface,
		PrivateKey:    privKey,
		PublicKey:     pubKey,
		Subnet:        subnet,
		DNS:           dnsPtr,
		External:      false,
		IsActive:      true,
	}

	// Setup the interface on the OS
	if err := SetupInterface(iface, subnet, port, privKey); err != nil {
		return nil, fmt.Errorf("setup interface: %w", err)
	}

	// Configure private key + port via wgctrl
	mgr, err := NewManager(iface)
	if err != nil {
		TeardownInterface(iface)
		return nil, fmt.Errorf("wgctrl: %w", err)
	}
	if err := mgr.ConfigureInterface(privKey, port); err != nil {
		mgr.Close()
		TeardownInterface(iface)
		return nil, fmt.Errorf("configure interface: %w", err)
	}

	// Ensure firewall for this subnet
	if err := r.fw.EnsureSubnet(subnet); err != nil {
		log.Printf("warn: firewall subnet %s: %v", subnet, err)
	}

	// Persist to DB
	_, err = r.db.Exec(`
		INSERT INTO wg_servers (id, name, endpoint, port, interface_name, private_key, public_key, subnet, dns, external, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, NOW())`,
		srv.ID, srv.Name, srv.Endpoint, srv.Port, srv.InterfaceName,
		srv.PrivateKey, srv.PublicKey, srv.Subnet, srv.DNS,
	)
	if err != nil {
		mgr.Close()
		TeardownInterface(iface)
		return nil, fmt.Errorf("insert server: %w", err)
	}

	// Init IP pool for this server
	if err := ippool.InitServer(r.db, srv.ID, subnet); err != nil {
		log.Printf("warn: init pool for %s: %v", srv.ID, err)
	}

	r.mu.Lock()
	r.servers[srv.ID] = &ServerInstance{Server: srv, Manager: mgr}
	r.mu.Unlock()

	return &srv, nil
}

// CreateExternal registers a pre-existing (externally managed) WireGuard interface.
func (r *Registry) CreateExternal(name, endpoint string, port int, iface, pubKey, subnet, dns string) (*model.WGServer, error) {
	var dnsPtr *string
	if dns != "" {
		dnsPtr = &dns
	}

	srv := model.WGServer{
		ID:            uuid.New().String(),
		Name:          name,
		Endpoint:      endpoint,
		Port:          port,
		InterfaceName: iface,
		PrivateKey:    "",
		PublicKey:     pubKey,
		Subnet:        subnet,
		DNS:           dnsPtr,
		External:      true,
		IsActive:      true,
	}

	mgr, err := NewManager(iface)
	if err != nil {
		return nil, fmt.Errorf("wgctrl: %w", err)
	}

	if err := r.fw.EnsureSubnet(subnet); err != nil {
		log.Printf("warn: firewall subnet %s: %v", subnet, err)
	}

	_, err = r.db.Exec(`
		INSERT INTO wg_servers (id, name, endpoint, port, interface_name, private_key, public_key, subnet, dns, external, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, 1, 1, NOW())`,
		srv.ID, srv.Name, srv.Endpoint, srv.Port, srv.InterfaceName,
		srv.PublicKey, srv.Subnet, srv.DNS,
	)
	if err != nil {
		mgr.Close()
		return nil, fmt.Errorf("insert external server: %w", err)
	}

	if err := ippool.InitServer(r.db, srv.ID, subnet); err != nil {
		log.Printf("warn: init pool for %s: %v", srv.ID, err)
	}

	r.mu.Lock()
	r.servers[srv.ID] = &ServerInstance{Server: srv, Manager: mgr}
	r.mu.Unlock()

	return &srv, nil
}

// Delete terminates all sessions on a server and removes it.
// Tears down the kernel interface and firewall rules even if the server
// wasn't in the runtime map (e.g., it was orphaned because startServer failed).
func (r *Registry) Delete(serverID string) error {
	// Snapshot the DB row up-front so we know the iface/subnet/external flag
	// regardless of whether the server is currently running.
	var srv model.WGServer
	hasRow := r.db.Get(&srv, "SELECT * FROM wg_servers WHERE id=?", serverID) == nil

	r.mu.Lock()
	inst, ok := r.servers[serverID]
	if ok {
		delete(r.servers, serverID)
	}
	r.mu.Unlock()

	if ok {
		inst.Manager.Close()
	}
	// Always attempt to teardown using DB metadata — handles the orphaned-server case.
	if hasRow && !srv.External {
		TeardownInterface(srv.InterfaceName)
	}
	if hasRow {
		r.fw.RemoveSubnet(srv.Subnet)
	}

	// Cascade: orphan sessions reference a non-existent server otherwise.
	r.db.Exec("DELETE FROM sessions WHERE server_id=?", serverID)
	r.db.Exec("DELETE FROM ip_pool WHERE server_id=?", serverID)
	_, err := r.db.Exec("DELETE FROM wg_servers WHERE id=?", serverID)
	return err
}

// Update updates a server's metadata.
func (r *Registry) Update(serverID, name, endpoint string, port int, dns string) error {
	var dnsVal interface{} = nil
	if dns != "" {
		dnsVal = dns
	}
	if port <= 0 {
		port = 51820
	}
	_, err := r.db.Exec("UPDATE wg_servers SET name=?, endpoint=?, port=?, dns=? WHERE id=?",
		name, endpoint, port, dnsVal, serverID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	if inst, ok := r.servers[serverID]; ok {
		inst.Server.Name = name
		inst.Server.Endpoint = endpoint
		inst.Server.Port = port
		if dns != "" {
			inst.Server.DNS = &dns
		} else {
			inst.Server.DNS = nil
		}
	}
	r.mu.Unlock()
	return nil
}

// GroupServers returns servers assigned to a group.
func (r *Registry) GroupServers(groupID string) ([]model.WGServer, error) {
	var servers []model.WGServer
	err := r.db.Select(&servers,
		`SELECT s.* FROM wg_servers s
		 JOIN group_server_access g ON g.server_id=s.id
		 WHERE g.group_id=? ORDER BY s.name`, groupID)
	return servers, err
}

// AddGroupServer assigns a server to a group.
func (r *Registry) AddGroupServer(groupID, serverID string) error {
	_, err := r.db.Exec(
		"INSERT IGNORE INTO group_server_access (group_id, server_id) VALUES (?, ?)",
		groupID, serverID)
	return err
}

// RemoveGroupServer removes a server from a group.
func (r *Registry) RemoveGroupServer(groupID, serverID string) error {
	_, err := r.db.Exec(
		"DELETE FROM group_server_access WHERE group_id=? AND server_id=?",
		groupID, serverID)
	return err
}

// UserDirectServers returns servers directly assigned to a user.
func (r *Registry) UserDirectServers(userID string) ([]model.WGServer, error) {
	var servers []model.WGServer
	err := r.db.Select(&servers,
		`SELECT s.* FROM wg_servers s
		 JOIN user_server_access u ON u.server_id=s.id
		 WHERE u.user_id=? ORDER BY s.name`, userID)
	return servers, err
}

// AddUserServer directly assigns a server to a user.
func (r *Registry) AddUserServer(userID, serverID string) error {
	_, err := r.db.Exec(
		"INSERT IGNORE INTO user_server_access (user_id, server_id) VALUES (?, ?)",
		userID, serverID)
	return err
}

// RemoveUserServer removes a direct user-server assignment.
func (r *Registry) RemoveUserServer(userID, serverID string) error {
	_, err := r.db.Exec(
		"DELETE FROM user_server_access WHERE user_id=? AND server_id=?",
		userID, serverID)
	return err
}

// AllFromDB returns all servers from the database (including inactive).
func (r *Registry) AllFromDB() ([]model.WGServer, error) {
	var servers []model.WGServer
	err := r.db.Select(&servers, "SELECT * FROM wg_servers ORDER BY name")
	return servers, err
}

// startServer brings up a single server (called at Start time).
func (r *Registry) startServer(srv *model.WGServer) error {
	if !srv.External {
		if err := SetupInterface(srv.InterfaceName, srv.Subnet, srv.Port, srv.PrivateKey); err != nil {
			return fmt.Errorf("setup interface: %w", err)
		}
	}

	mgr, err := NewManager(srv.InterfaceName)
	if err != nil {
		return fmt.Errorf("wgctrl: %w", err)
	}

	if !srv.External && srv.PrivateKey != "" {
		if err := mgr.ConfigureInterface(srv.PrivateKey, srv.Port); err != nil {
			log.Printf("warn: configure interface %s: %v", srv.InterfaceName, err)
		}
	}

	if err := r.fw.EnsureSubnet(srv.Subnet); err != nil {
		log.Printf("warn: firewall subnet %s: %v", srv.Subnet, err)
	}

	// Init pool if empty for this server
	var count int
	r.db.Get(&count, "SELECT COUNT(*) FROM ip_pool WHERE server_id=?", srv.ID)
	if count == 0 {
		if err := ippool.InitServer(r.db, srv.ID, srv.Subnet); err != nil {
			log.Printf("warn: init pool server %s: %v", srv.ID, err)
		}
	}

	// Re-attach existing session peers from the DB. Without this, every
	// daemon restart silently breaks active client sessions until they
	// time out — clients show "Connected" but no traffic flows.
	var sessionRows []struct {
		ClientPublicKey string `db:"client_public_key"`
		AssignedIP      string `db:"assigned_ip"`
	}
	if err := r.db.Select(&sessionRows,
		"SELECT client_public_key, assigned_ip FROM sessions WHERE server_id=?", srv.ID); err == nil && len(sessionRows) > 0 {
		peers := make([]PeerEntry, 0, len(sessionRows))
		for _, row := range sessionRows {
			peers = append(peers, PeerEntry{PublicKey: row.ClientPublicKey, AssignedIP: row.AssignedIP})
		}
		if err := mgr.SyncAllPeers(peers); err != nil {
			log.Printf("warn: re-sync peers for %s: %v", srv.InterfaceName, err)
		} else {
			log.Printf("re-attached %d existing peer(s) on %s", len(peers), srv.InterfaceName)
		}
	}

	r.mu.Lock()
	r.servers[srv.ID] = &ServerInstance{Server: *srv, Manager: mgr}
	r.mu.Unlock()
	return nil
}
