package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/model"
)

type ResourceHandler struct{ DB *sqlx.DB }

func (h *ResourceHandler) List(w http.ResponseWriter, r *http.Request) {
	resources := []model.Resource{}
	h.DB.Select(&resources, "SELECT * FROM resources ORDER BY name")
	jsonOK(w, resources)
}

func (h *ResourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		IPAddress   string  `json:"ip_address"`
		Type        string  `json:"type"`
		Mask        *int    `json:"mask"`
		Ports       *string `json:"ports"`
		Description *string `json:"description"`
	}
	if err := decode(r, &req); err != nil || req.Name == "" || req.IPAddress == "" {
		jsonError(w, 400, "name and ip_address required")
		return
	}
	if req.Type != "host" && req.Type != "network" {
		req.Type = "host"
	}
	id := uuid.New().String()
	h.DB.Exec(
		"INSERT INTO resources (id, name, ip_address, type, mask, ports, description) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, req.Name, req.IPAddress, req.Type, req.Mask, req.Ports, req.Description,
	)
	var res model.Resource
	h.DB.Get(&res, "SELECT * FROM resources WHERE id=?", id)
	jsonOK(w, res)
}

func (h *ResourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var res model.Resource
	if err := h.DB.Get(&res, "SELECT * FROM resources WHERE id=?", id); err != nil {
		jsonError(w, 404, "not found")
		return
	}
	jsonOK(w, res)
}

func (h *ResourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		IPAddress   *string `json:"ip_address"`
		Type        *string `json:"type"`
		Mask        *int    `json:"mask"`
		Ports       *string `json:"ports"`
		Description *string `json:"description"`
	}
	decode(r, &req)
	if req.Name != nil {
		h.DB.Exec("UPDATE resources SET name=? WHERE id=?", *req.Name, id)
	}
	if req.IPAddress != nil {
		h.DB.Exec("UPDATE resources SET ip_address=? WHERE id=?", *req.IPAddress, id)
	}
	if req.Type != nil {
		h.DB.Exec("UPDATE resources SET type=? WHERE id=?", *req.Type, id)
	}
	if req.Mask != nil {
		h.DB.Exec("UPDATE resources SET mask=? WHERE id=?", *req.Mask, id)
	}
	if req.Ports != nil {
		h.DB.Exec("UPDATE resources SET ports=? WHERE id=?", *req.Ports, id)
	}
	if req.Description != nil {
		h.DB.Exec("UPDATE resources SET description=? WHERE id=?", *req.Description, id)
	}
	var res model.Resource
	h.DB.Get(&res, "SELECT * FROM resources WHERE id=?", id)
	jsonOK(w, res)
}

func (h *ResourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec("DELETE FROM resources WHERE id=?", id)
	jsonOK(w, map[string]bool{"ok": true})
}

// Resource Groups

type ResourceGroupHandler struct{ DB *sqlx.DB }

func (h *ResourceGroupHandler) List(w http.ResponseWriter, r *http.Request) {
	rgs := []model.ResourceGroup{}
	h.DB.Select(&rgs, "SELECT * FROM resource_groups ORDER BY name")
	jsonOK(w, rgs)
}

func (h *ResourceGroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := decode(r, &req); err != nil || req.Name == "" {
		jsonError(w, 400, "name required")
		return
	}
	id := uuid.New().String()
	h.DB.Exec("INSERT INTO resource_groups (id, name, description) VALUES (?, ?, ?)", id, req.Name, req.Description)
	var rg model.ResourceGroup
	h.DB.Get(&rg, "SELECT * FROM resource_groups WHERE id=?", id)
	jsonOK(w, rg)
}

func (h *ResourceGroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rg model.ResourceGroup
	if err := h.DB.Get(&rg, "SELECT * FROM resource_groups WHERE id=?", id); err != nil {
		jsonError(w, 404, "not found")
		return
	}
	// Include members
	members := []model.Resource{}
	h.DB.Select(&members, `
		SELECT r.* FROM resources r
		JOIN resource_group_members rgm ON rgm.resource_id = r.id
		WHERE rgm.resource_group_id = ?`, id)
	jsonOK(w, map[string]any{"group": rg, "resources": members})
}

func (h *ResourceGroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	decode(r, &req)
	if req.Name != nil {
		h.DB.Exec("UPDATE resource_groups SET name=? WHERE id=?", *req.Name, id)
	}
	if req.Description != nil {
		h.DB.Exec("UPDATE resource_groups SET description=? WHERE id=?", *req.Description, id)
	}
	var rg model.ResourceGroup
	h.DB.Get(&rg, "SELECT * FROM resource_groups WHERE id=?", id)
	jsonOK(w, rg)
}

func (h *ResourceGroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec("DELETE FROM resource_groups WHERE id=?", id)
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *ResourceGroupHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	rgID := chi.URLParam(r, "id")
	var req struct{ ResourceID string `json:"resource_id"` }
	decode(r, &req)
	h.DB.Exec("INSERT IGNORE INTO resource_group_members (resource_group_id, resource_id) VALUES (?, ?)",
		rgID, req.ResourceID)
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *ResourceGroupHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	rgID := chi.URLParam(r, "id")
	resID := chi.URLParam(r, "rid")
	h.DB.Exec("DELETE FROM resource_group_members WHERE resource_group_id=? AND resource_id=?", rgID, resID)
	jsonOK(w, map[string]bool{"ok": true})
}

// ListServers GET /admin/resource-groups/{id}/servers — servers this bundle is attached to.
func (h *ResourceGroupHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	rgID := chi.URLParam(r, "id")
	type srv struct {
		ID     string `db:"id"     json:"id"`
		Name   string `db:"name"   json:"name"`
		Subnet string `db:"subnet" json:"subnet"`
	}
	var rows []srv
	if err := h.DB.Select(&rows, `
		SELECT s.id, s.name, s.subnet
		FROM wg_servers s
		JOIN server_bundle_access sba ON sba.server_id = s.id
		WHERE sba.bundle_id = ? AND s.is_active = 1
		ORDER BY s.name`, rgID); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// AttachServer POST /admin/resource-groups/{id}/servers  body: {server_id}
func (h *ResourceGroupHandler) AttachServer(w http.ResponseWriter, r *http.Request) {
	rgID := chi.URLParam(r, "id")
	var req struct{ ServerID string `json:"server_id"` }
	if err := decode(r, &req); err != nil || req.ServerID == "" {
		jsonError(w, 400, "server_id required")
		return
	}
	if _, err := h.DB.Exec(
		"INSERT IGNORE INTO server_bundle_access (server_id, bundle_id) VALUES (?, ?)",
		req.ServerID, rgID,
	); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DetachServer DELETE /admin/resource-groups/{id}/servers/{sid}
func (h *ResourceGroupHandler) DetachServer(w http.ResponseWriter, r *http.Request) {
	rgID := chi.URLParam(r, "id")
	sID := chi.URLParam(r, "sid")
	h.DB.Exec("DELETE FROM server_bundle_access WHERE server_id=? AND bundle_id=?", sID, rgID)
	jsonOK(w, map[string]bool{"ok": true})
}
