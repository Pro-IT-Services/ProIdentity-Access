package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/model"
	"proidentity/internal/wireguard"
)

// ServerHandler handles WireGuard server CRUD and assignment operations.
type ServerHandler struct {
	Registry *wireguard.Registry
	DB       *sqlx.DB
}

// List GET /admin/servers
// Returns each DB row decorated with `running: bool` so the admin panel can
// distinguish servers that are actually live from ones whose interface failed
// to come up at boot or were marked inactive.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.Registry.AllFromDB()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	runningIDs := map[string]bool{}
	for _, inst := range h.Registry.All() {
		runningIDs[inst.Server.ID] = true
	}
	out := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		out = append(out, safeServerResponse(s, runningIDs[s.ID]))
	}
	jsonOK(w, out)
}

// Create POST /admin/servers
func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Endpoint  string `json:"endpoint"`
		Port      int    `json:"port"`
		Iface     string `json:"interface_name"`
		Subnet    string `json:"subnet"`
		DNS       string `json:"dns"`
		External  bool   `json:"external"`
		PublicKey string `json:"public_key"` // only for external
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	if req.Name == "" || req.Endpoint == "" || req.Iface == "" || req.Subnet == "" {
		jsonError(w, 400, "name, endpoint, interface_name, and subnet are required")
		return
	}
	if req.Port == 0 {
		req.Port = 51820
	}

	if req.External {
		if req.PublicKey == "" {
			jsonError(w, 400, "public_key required for external server")
			return
		}
		srv, err := h.Registry.CreateExternal(req.Name, req.Endpoint, req.Port, req.Iface, req.PublicKey, req.Subnet, req.DNS)
		if err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		jsonOK(w, safeServerResponse(*srv, true))
		return
	}

	srv, err := h.Registry.Create(req.Name, req.Endpoint, req.Port, req.Iface, req.Subnet, req.DNS)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, safeServerResponse(*srv, true))
}

// Get GET /admin/servers/{id}
func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	servers, err := h.Registry.AllFromDB()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	for _, s := range servers {
		if s.ID == id {
			_, running := h.Registry.Get(s.ID)
			jsonOK(w, safeServerResponse(s, running == nil))
			return
		}
	}
	jsonError(w, 404, "server not found")
}

func safeServerResponse(s model.WGServer, running bool) map[string]any {
	return map[string]any{
		"id":             s.ID,
		"name":           s.Name,
		"endpoint":       s.Endpoint,
		"port":           s.Port,
		"interface_name": s.InterfaceName,
		"subnet":         s.Subnet,
		"dns":            s.DNS,
		"external":       s.External,
		"is_active":      s.IsActive,
		"created_at":     s.CreatedAt,
		"running":        running,
	}
}

// Update PUT /admin/servers/{id}
func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name     string `json:"name"`
		Endpoint string `json:"endpoint"`
		DNS      string `json:"dns"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	if err := h.Registry.Update(id, req.Name, req.Endpoint, req.DNS); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// Delete DELETE /admin/servers/{id}
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Registry.Delete(id); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// ListGroups GET /admin/servers/{id}/groups — groups that have access to this server
func (h *ServerHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var groups []model.Group
	err := h.DB.Select(&groups,
		`SELECT g.* FROM `+"`groups`"+` g
		 JOIN group_server_access gsa ON gsa.group_id = g.id
		 WHERE gsa.server_id = ? ORDER BY g.name`, serverID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, groups)
}

// AddGroup POST /admin/servers/{id}/groups
func (h *ServerHandler) AddGroup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var req struct {
		GroupID string `json:"group_id"`
	}
	if err := decode(r, &req); err != nil || req.GroupID == "" {
		jsonError(w, 400, "group_id required")
		return
	}
	if err := h.Registry.AddGroupServer(req.GroupID, serverID); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// RemoveGroup DELETE /admin/servers/{id}/groups/{gid}
func (h *ServerHandler) RemoveGroup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	groupID := chi.URLParam(r, "gid")
	if err := h.Registry.RemoveGroupServer(groupID, serverID); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// ListUsers GET /admin/servers/{id}/users — users directly assigned to this server
func (h *ServerHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var users []model.User
	err := h.DB.Select(&users,
		`SELECT u.* FROM users u
		 JOIN user_server_access usa ON usa.user_id = u.id
		 WHERE usa.server_id = ? ORDER BY u.username`, serverID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, users)
}

// AddUser POST /admin/servers/{id}/users
func (h *ServerHandler) AddUser(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decode(r, &req); err != nil || req.UserID == "" {
		jsonError(w, 400, "user_id required")
		return
	}
	if err := h.Registry.AddUserServer(req.UserID, serverID); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// RemoveUser DELETE /admin/servers/{id}/users/{uid}
func (h *ServerHandler) RemoveUser(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "uid")
	if err := h.Registry.RemoveUserServer(userID, serverID); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// ListBundles GET /admin/servers/{id}/bundles — bundles attached to this server.
func (h *ServerHandler) ListBundles(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	type bundle struct {
		ID          string  `db:"id"          json:"id"`
		Name        string  `db:"name"        json:"name"`
		Description *string `db:"description" json:"description"`
	}
	var rows []bundle
	err := h.DB.Select(&rows, `
		SELECT rg.id, rg.name, rg.description
		FROM resource_groups rg
		JOIN server_bundle_access sba ON sba.bundle_id = rg.id
		WHERE sba.server_id = ?
		ORDER BY rg.name`, serverID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// AddBundle POST /admin/servers/{id}/bundles  body: {bundle_id}
func (h *ServerHandler) AddBundle(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	var req struct {
		BundleID string `json:"bundle_id"`
	}
	if err := decode(r, &req); err != nil || req.BundleID == "" {
		jsonError(w, 400, "bundle_id required")
		return
	}
	if _, err := h.DB.Exec(
		"INSERT IGNORE INTO server_bundle_access (server_id, bundle_id) VALUES (?, ?)",
		serverID, req.BundleID,
	); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// RemoveBundle DELETE /admin/servers/{id}/bundles/{bid}
func (h *ServerHandler) RemoveBundle(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	bundleID := chi.URLParam(r, "bid")
	if _, err := h.DB.Exec(
		"DELETE FROM server_bundle_access WHERE server_id = ? AND bundle_id = ?",
		serverID, bundleID,
	); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
