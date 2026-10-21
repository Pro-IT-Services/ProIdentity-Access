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
	"vpn_name":              true,
	"session_timeout":       true,
	"keepalive_interval":    true,
	"webauthn_rp_id":        true,
	"webauthn_rp_name":      true,
	"webauthn_origin":       true,
	"push_auth_enabled":     true,
	"push_auth_api_key":     true,
	"allow_personal_import": true,
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

	// Turning push auth on (or changing its key) contacts nobody: the only
	// way the ProIdentity Cloud API can create an account is an approval
	// request, which would push to every enrolled user. Accounts are created
	// on each person's first real sign-in or connection instead.
	jsonOK(w, map[string]any{"ok": true})
}
