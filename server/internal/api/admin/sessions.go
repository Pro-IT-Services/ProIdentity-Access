package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

type SessionHandler struct{ DB *sqlx.DB }

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Queryx(`
		SELECT s.id, s.assigned_ip, s.created_at, s.last_keepalive,
		       u.username, u.email
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		ORDER BY s.created_at DESC`)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var sessions []map[string]any
	for rows.Next() {
		m := map[string]any{}
		rows.MapScan(m)
		// Convert []byte values to string for JSON
		for k, v := range m {
			if b, ok := v.([]byte); ok {
				m[k] = string(b)
			}
		}
		sessions = append(sessions, m)
	}
	if sessions == nil {
		sessions = []map[string]any{}
	}
	jsonOK(w, sessions)
}

func (h *SessionHandler) Terminate(w http.ResponseWriter, r *http.Request) {
	// The actual teardown is handled by session.Manager; here we just return
	// the session ID for the router to process with the session manager.
	// This endpoint is wired in router.go to call sessions.Terminate directly.
	id := chi.URLParam(r, "id")
	jsonOK(w, map[string]string{"session_id": id, "action": "terminate"})
}
