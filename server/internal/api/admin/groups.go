package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"proidentity/internal/model"
)

type GroupHandler struct{ DB *sqlx.DB }

// listRow extends model.Group with permissions decoded from JSON.
type listRow struct {
	model.Group
	PermissionsRaw *string `db:"permissions"`
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	rows := []listRow{}
	h.DB.Select(&rows, "SELECT *, permissions FROM `groups` ORDER BY name")
	out := make([]map[string]any, 0, len(rows))
	for _, gr := range rows {
		var perms []string
		if gr.PermissionsRaw != nil && *gr.PermissionsRaw != "" {
			_ = json.Unmarshal([]byte(*gr.PermissionsRaw), &perms)
		}
		out = append(out, map[string]any{
			"id":          gr.ID,
			"name":        gr.Name,
			"description": gr.Description,
			"created_at":  gr.CreatedAt,
			"permissions": perms,
		})
	}
	jsonOK(w, out)
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := decode(r, &req); err != nil || req.Name == "" {
		jsonError(w, 400, "name required")
		return
	}
	id := uuid.New().String()
	_, err := h.DB.Exec("INSERT INTO `groups` (id, name, description) VALUES (?, ?, ?)",
		id, req.Name, req.Description)
	if err != nil {
		jsonError(w, 400, "name already exists")
		return
	}
	var g model.Group
	h.DB.Get(&g, "SELECT * FROM `groups` WHERE id=?", id)
	jsonOK(w, g)
}

func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var g model.Group
	if err := h.DB.Get(&g, "SELECT * FROM `groups` WHERE id=?", id); err != nil {
		jsonError(w, 404, "not found")
		return
	}
	jsonOK(w, g)
}

func (h *GroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	decode(r, &req)
	if req.Name != nil {
		h.DB.Exec("UPDATE `groups` SET name=? WHERE id=?", *req.Name, id)
	}
	if req.Description != nil {
		h.DB.Exec("UPDATE `groups` SET description=? WHERE id=?", *req.Description, id)
	}
	var g model.Group
	h.DB.Get(&g, "SELECT * FROM `groups` WHERE id=?", id)
	jsonOK(w, g)
}

func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec("DELETE FROM `groups` WHERE id=?", id)
	jsonOK(w, map[string]bool{"ok": true})
}

// PUT /admin/groups/{id}/permissions  body: {"permissions": ["users.manage", ...]}
func (h *GroupHandler) UpdatePermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, "invalid body")
		return
	}
	if req.Permissions == nil {
		req.Permissions = []string{}
	}
	b, err := json.Marshal(req.Permissions)
	if err != nil {
		jsonError(w, 500, "marshal")
		return
	}
	if _, err := h.DB.Exec("UPDATE `groups` SET permissions=? WHERE id=?", string(b), id); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// Resource-group access management
func (h *GroupHandler) ListAccess(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rgs := []model.ResourceGroup{}
	h.DB.Select(&rgs, `
		SELECT rg.* FROM resource_groups rg
		JOIN group_access ga ON ga.resource_group_id = rg.id
		WHERE ga.group_id = ?`, id)
	jsonOK(w, rgs)
}

func (h *GroupHandler) AddAccess(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	var req struct{ ResourceGroupID string `json:"resource_group_id"` }
	decode(r, &req)
	h.DB.Exec("INSERT IGNORE INTO group_access (group_id, resource_group_id) VALUES (?, ?)",
		groupID, req.ResourceGroupID)
	jsonOK(w, map[string]bool{"ok": true})
}

func (h *GroupHandler) RemoveAccess(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	rgID := chi.URLParam(r, "rgid")
	h.DB.Exec("DELETE FROM group_access WHERE group_id=? AND resource_group_id=?", groupID, rgID)
	jsonOK(w, map[string]bool{"ok": true})
}
