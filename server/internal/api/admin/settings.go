package admin

import (
	"net/http"
	"strings"

	"github.com/jmoiron/sqlx"
)

type SettingsHandler struct{ DB *sqlx.DB }

const configuredSecretMarker = "__configured__"

var secretSettings = map[string]bool{
	"push_auth_api_key": true,
}

var visibleSettings = map[string]bool{
	"vpn_name":           true,
	"session_timeout":    true,
	"keepalive_interval": true,
	"webauthn_rp_id":     true,
	"webauthn_rp_name":   true,
	"webauthn_origin":    true,
	"push_auth_enabled":  true,
	"push_auth_api_key":  true,
}

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
		if !visibleSettings[k] {
			continue
		}
		if secretSettings[k] && strings.TrimSpace(v) != "" {
			v = configuredSecretMarker
		}
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
		normalized, err := validateSettingValue(k, v)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		if secretSettings[k] && normalized == "" {
			continue
		}
		if _, err := h.DB.Exec("INSERT INTO settings (`key`, `value`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `value`=?", k, normalized, normalized); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
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
