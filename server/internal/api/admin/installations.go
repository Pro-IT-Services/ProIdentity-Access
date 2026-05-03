package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type InstallationHandler struct{ DB *sqlx.DB }

type installationRow struct {
	ID         string     `db:"id"          json:"id"`
	DeviceName string     `db:"device_name" json:"device_name"`
	UserID     *string    `db:"user_id"     json:"user_id"`
	Username   string     `db:"username"    json:"username"`
	IsActive   bool       `db:"is_active"   json:"is_active"`
	LastSeen   *time.Time `db:"last_seen"   json:"last_seen"`
	CreatedAt  time.Time  `db:"created_at"  json:"created_at"`
}

func (h *InstallationHandler) List(w http.ResponseWriter, r *http.Request) {
	var insts []installationRow
	h.DB.Select(&insts, `
		SELECT i.id, i.device_name, i.user_id, i.is_active, i.last_seen, i.created_at,
		       COALESCE(u.username, '') AS username
		FROM installations i
		LEFT JOIN users u ON u.id = i.user_id
		ORDER BY i.created_at DESC
	`)
	if insts == nil {
		insts = []installationRow{}
	}
	jsonOK(w, insts)
}

func (h *InstallationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.DB.Exec("UPDATE installations SET is_active=0 WHERE id=?", id)
	jsonOK(w, map[string]bool{"ok": true})
}
