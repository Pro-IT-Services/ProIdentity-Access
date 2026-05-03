package admin

import (
	"net/http"

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
	for k, v := range req {
		h.DB.Exec("INSERT INTO settings (`key`, `value`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `value`=?", k, v, v)
	}
	jsonOK(w, map[string]bool{"ok": true})
}
