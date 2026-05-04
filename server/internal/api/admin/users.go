package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/auth"
	"proidentity/internal/model"
)

type UserHandler struct{ DB *sqlx.DB }

func (h *UserHandler) canManageAdminTarget(r *http.Request, userID string) bool {
	claims := claimsFrom(r)
	if claims != nil && claims.IsAdmin {
		return true
	}
	var isAdmin bool
	if err := h.DB.Get(&isAdmin, "SELECT is_admin FROM users WHERE id=?", userID); err != nil {
		return false
	}
	return !isAdmin
}

func requireFullAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := claimsFrom(r)
	if claims == nil || !claims.IsAdmin {
		jsonError(w, http.StatusForbidden, "full admin required")
		return false
	}
	return true
}

// GET /api/v1/admin/users
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users := []model.User{}
	if err := h.DB.Select(&users, "SELECT * FROM users ORDER BY created_at DESC"); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, users)
}

// POST /api/v1/admin/users
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		IsAdmin   bool   `json:"is_admin"`
	}
	if err := decode(r, &req); err != nil || req.Username == "" || req.Password == "" {
		jsonError(w, 400, "username and password required")
		return
	}
	if req.IsAdmin && !requireFullAdmin(w, r) {
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, 500, "hash error")
		return
	}
	id := uuid.New().String()
	_, err = h.DB.Exec(`
		INSERT INTO users (id, username, email, first_name, last_name, password_hash, is_admin, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
		id, req.Username, req.Email, req.FirstName, req.LastName, hash, req.IsAdmin)
	if err != nil {
		jsonError(w, 400, "username or email already exists")
		return
	}
	var user model.User
	h.DB.Get(&user, "SELECT * FROM users WHERE id=?", id)
	maybeEnsurePushAuthUser(h.DB, user, r)
	jsonOK(w, user)
}

// GET /api/v1/admin/users/{id}
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var user model.User
	if err := h.DB.Get(&user, "SELECT * FROM users WHERE id=?", id); err != nil {
		jsonError(w, 404, "not found")
		return
	}
	jsonOK(w, user)
}

// PUT /api/v1/admin/users/{id}
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Email         *string `json:"email"`
		Password      *string `json:"password"`
		FirstName     *string `json:"first_name"`
		LastName      *string `json:"last_name"`
		IsAdmin       *bool   `json:"is_admin"`
		IsActive      *bool   `json:"is_active"`
		DisableTOTP   *bool   `json:"disable_totp"`
		AdminPassword *string `json:"admin_password"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	if !h.canManageAdminTarget(r, id) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	if req.Password != nil {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			jsonError(w, 500, "hash error")
			return
		}
		h.DB.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, id)
	}
	if req.Email != nil {
		h.DB.Exec("UPDATE users SET email=? WHERE id=?", *req.Email, id)
	}
	if req.FirstName != nil {
		h.DB.Exec("UPDATE users SET first_name=? WHERE id=?", *req.FirstName, id)
	}
	if req.LastName != nil {
		h.DB.Exec("UPDATE users SET last_name=? WHERE id=?", *req.LastName, id)
	}
	if req.IsAdmin != nil {
		if !requireFullAdmin(w, r) {
			return
		}
		h.DB.Exec("UPDATE users SET is_admin=? WHERE id=?", *req.IsAdmin, id)
	}
	if req.IsActive != nil {
		h.DB.Exec("UPDATE users SET is_active=? WHERE id=?", *req.IsActive, id)
	}
	if req.DisableTOTP != nil && *req.DisableTOTP {
		if req.AdminPassword == nil || *req.AdminPassword == "" {
			jsonError(w, 400, "admin_password required to disable 2FA")
			return
		}
		claims := claimsFrom(r)
		if claims == nil {
			jsonError(w, 401, "unauthorized")
			return
		}
		var adminHash string
		if err := h.DB.Get(&adminHash, "SELECT password_hash FROM users WHERE id=?", claims.UserID); err != nil {
			jsonError(w, 500, "admin lookup failed")
			return
		}
		if !auth.CheckPassword(adminHash, *req.AdminPassword) {
			jsonError(w, 401, "invalid admin password")
			return
		}
		h.DB.Exec("UPDATE users SET totp_enabled=0, totp_secret=NULL WHERE id=?", id)
	}
	var user model.User
	h.DB.Get(&user, "SELECT * FROM users WHERE id=?", id)
	jsonOK(w, user)
}

// DELETE /api/v1/admin/users/{id}
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, id) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	h.DB.Exec("DELETE FROM users WHERE id=?", id)
	jsonOK(w, map[string]bool{"ok": true})
}

// GET /api/v1/admin/users/{id}/groups
func (h *UserHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	groups := []model.Group{}
	h.DB.Select(&groups, `
		SELECT g.* FROM `+"`groups`"+` g
		JOIN user_groups ug ON ug.group_id = g.id
		WHERE ug.user_id = ?`, id)
	jsonOK(w, groups)
}

// POST /api/v1/admin/users/{id}/groups
func (h *UserHandler) AddGroup(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	var req struct {
		GroupID string `json:"group_id"`
	}
	decode(r, &req)
	h.DB.Exec("INSERT IGNORE INTO user_groups (user_id, group_id) VALUES (?, ?)", userID, req.GroupID)
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/users/{id}/groups/{gid}
func (h *UserHandler) RemoveGroup(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	groupID := chi.URLParam(r, "gid")
	h.DB.Exec("DELETE FROM user_groups WHERE user_id=? AND group_id=?", userID, groupID)
	jsonOK(w, map[string]bool{"ok": true})
}

// ListServers GET /admin/users/{id}/servers — servers this user can connect to.
func (h *UserHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	type srv struct {
		ID     string `db:"id"     json:"id"`
		Name   string `db:"name"   json:"name"`
		Subnet string `db:"subnet" json:"subnet"`
	}
	rows := []srv{}
	err := h.DB.Select(&rows, `
		SELECT s.id, s.name, s.subnet
		FROM wg_servers s
		JOIN user_server_access usa ON usa.server_id = s.id
		WHERE usa.user_id = ? AND s.is_active = 1
		ORDER BY s.name`, userID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// AddServer POST /admin/users/{id}/servers  body: {server_id}
func (h *UserHandler) AddServer(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
	}
	if err := decode(r, &req); err != nil || req.ServerID == "" {
		jsonError(w, 400, "server_id required")
		return
	}
	h.DB.Exec("INSERT IGNORE INTO user_server_access (user_id, server_id) VALUES (?, ?)", userID, req.ServerID)
	jsonOK(w, map[string]bool{"ok": true})
}

// RemoveServer DELETE /admin/users/{id}/servers/{sid}
func (h *UserHandler) RemoveServer(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	sID := chi.URLParam(r, "sid")
	h.DB.Exec("DELETE FROM user_server_access WHERE user_id=? AND server_id=?", userID, sID)
	jsonOK(w, map[string]bool{"ok": true})
}

// reachRow is one (resource, bundle, server) tuple in the user's reach set.
// The Role* fields are kept (always nil) for transitional UI compatibility.
type reachRow struct {
	ResourceID   string  `db:"resource_id"   json:"resource_id"`
	ResourceName string  `db:"resource_name" json:"resource_name"`
	IPAddress    string  `db:"ip_address"    json:"ip_address"`
	Type         string  `db:"resource_type" json:"type"`
	Mask         *int    `db:"mask"          json:"mask"`
	Ports        *string `db:"ports"         json:"ports"`
	BundleID     string  `db:"bundle_id"     json:"bundle_id"`
	BundleName   string  `db:"bundle_name"   json:"bundle_name"`
	ServerID     string  `db:"server_id"     json:"server_id"`
	ServerName   string  `db:"server_name"   json:"server_name"`
	ServerSubnet string  `db:"server_subnet" json:"server_subnet"`
}

// GET /api/v1/admin/users/{id}/reach — flattened access set with provenance.
// Flow: Person -> Server -> user_bundle_access -> Bundle -> Resource.
func (h *UserHandler) Reach(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	rows := []reachRow{}
	err := h.DB.Select(&rows, `
		SELECT
			r.id          AS resource_id,
			r.name        AS resource_name,
			r.ip_address  AS ip_address,
			r.type        AS resource_type,
			r.mask        AS mask,
			r.ports       AS ports,
			rg.id         AS bundle_id,
			rg.name       AS bundle_name,
			s.id          AS server_id,
			s.name        AS server_name,
			s.subnet      AS server_subnet
		FROM resources r
		JOIN resource_group_members rgm ON rgm.resource_id = r.id
		JOIN resource_groups rg         ON rg.id = rgm.resource_group_id
		JOIN user_bundle_access uba     ON uba.bundle_id = rg.id
		JOIN wg_servers s               ON s.id = uba.server_id AND s.is_active = 1
		WHERE uba.user_id = ?
		ORDER BY s.name, rg.name, r.name`, userID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// ListBundles GET /admin/users/{id}/servers/{sid}/bundles — bundles assigned to this user on this server.
func (h *UserHandler) ListBundles(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	serverID := chi.URLParam(r, "sid")
	type row struct {
		ID   string `db:"id"   json:"id"`
		Name string `db:"name" json:"name"`
		Desc string `db:"description" json:"description"`
	}
	var rows []row
	err := h.DB.Select(&rows, `
		SELECT rg.id, rg.name, rg.description
		FROM resource_groups rg
		JOIN user_bundle_access uba ON uba.bundle_id = rg.id
		WHERE uba.user_id = ? AND uba.server_id = ?
		ORDER BY rg.name`, userID, serverID)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// AddBundle POST /admin/users/{id}/servers/{sid}/bundles  body: {bundle_id}
func (h *UserHandler) AddBundle(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	serverID := chi.URLParam(r, "sid")
	var req struct {
		BundleID string `json:"bundle_id"`
	}
	if err := decode(r, &req); err != nil || req.BundleID == "" {
		jsonError(w, 400, "bundle_id required")
		return
	}
	// Validate: bundle must be allowed on this server.
	var count int
	h.DB.Get(&count, "SELECT COUNT(*) FROM server_bundle_access WHERE server_id=? AND bundle_id=?", serverID, req.BundleID)
	if count == 0 {
		jsonError(w, 400, "bundle is not allowed on this server")
		return
	}
	h.DB.Exec("INSERT IGNORE INTO user_bundle_access (user_id, server_id, bundle_id) VALUES (?, ?, ?)",
		userID, serverID, req.BundleID)
	jsonOK(w, map[string]bool{"ok": true})
}

// RemoveBundle DELETE /admin/users/{id}/servers/{sid}/bundles/{bid}
func (h *UserHandler) RemoveBundle(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if !h.canManageAdminTarget(r, userID) {
		jsonError(w, http.StatusForbidden, "full admin required")
		return
	}
	serverID := chi.URLParam(r, "sid")
	bundleID := chi.URLParam(r, "bid")
	h.DB.Exec("DELETE FROM user_bundle_access WHERE user_id=? AND server_id=? AND bundle_id=?",
		userID, serverID, bundleID)
	jsonOK(w, map[string]bool{"ok": true})
}
