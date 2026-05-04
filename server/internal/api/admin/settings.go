package admin

import (
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx"
)

type SettingsHandler struct{ DB *sqlx.DB }

// GET /api/v1/admin/settings
func (h *SettingsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Queryx("SELECT `key`, `value` FROM settings ORDER BY `key`")
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		result[k] = v
	}
	jsonOK(w, result)
}

// PUT /api/v1/admin/settings
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := decode(r, &req); err != nil {
		jsonError(w, 400, "bad request")
		return
	}
	oldEnabled, oldAPIKey := pushAuthSettings(h.DB)
	for k, v := range req {
		h.DB.Exec("INSERT INTO settings (`key`, `value`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `value`=?", k, v, v)
	}

	resp := map[string]any{"ok": true}
	newEnabled, newAPIKey := pushAuthSettings(h.DB)
	pushTouched := false
	if _, ok := req["push_auth_enabled"]; ok {
		pushTouched = true
	}
	if _, ok := req["push_auth_api_key"]; ok {
		pushTouched = true
	}
	if pushTouched && newEnabled && (!oldEnabled || strings.TrimSpace(oldAPIKey) != strings.TrimSpace(newAPIKey)) {
		resp["push_auth_sync"] = syncPushAuthUsers(h.DB, newAPIKey, requestIP(r))
	}
	jsonOK(w, resp)
}
