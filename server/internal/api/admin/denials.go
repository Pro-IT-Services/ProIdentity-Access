package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// DenialHandler exposes /admin/denials.
type DenialHandler struct{ DB *sqlx.DB }

// DenialRow is one row of the denied_attempts table, joined with username.
type DenialRow struct {
	ID       int64     `db:"id"        json:"id"`
	FirstTS  time.Time `db:"first_ts"  json:"first_ts"`
	LastTS   time.Time `db:"last_ts"   json:"last_ts"`
	Count    int       `db:"count"     json:"count"`
	UserID   *string   `db:"user_id"   json:"user_id"`
	Username *string   `db:"username"  json:"username"`
	SrcIP    string    `db:"src_ip"    json:"src_ip"`
	DstIP    string    `db:"dst_ip"    json:"dst_ip"`
	DstPort  *int      `db:"dst_port"  json:"dst_port"`
	Proto    string    `db:"proto"     json:"proto"`
}

// List GET /admin/denials?limit=&offset=&user_id=&since=&dst_ip=
func (h *DenialHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	where := []string{"1=1"}
	args := []any{}
	if v := q.Get("user_id"); v != "" {
		where = append(where, "d.user_id = ?")
		args = append(args, v)
	}
	if v := q.Get("dst_ip"); v != "" {
		where = append(where, "d.dst_ip = ?")
		args = append(args, v)
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			where = append(where, "d.last_ts >= ?")
			args = append(args, t)
		}
	}

	whereSQL := ""
	for i, c := range where {
		if i > 0 {
			whereSQL += " AND "
		}
		whereSQL += c
	}

	// COLLATE forces the JOIN to use a single collation across both tables —
	// without it MariaDB 11+ throws "Illegal mix of collations" when one side
	// uses uca1400_ai_ci (server default) and the other uses unicode_ci.
	sql := `
		SELECT d.id, d.first_ts, d.last_ts, d.count, d.user_id, d.src_ip, d.dst_ip, d.dst_port, d.proto,
		       u.username AS username
		FROM denied_attempts d
		LEFT JOIN users u ON u.id COLLATE utf8mb4_unicode_ci = d.user_id COLLATE utf8mb4_unicode_ci
		WHERE ` + whereSQL + `
		ORDER BY d.last_ts DESC
		LIMIT ? OFFSET ?`
	rows := []DenialRow{}
	if err := h.DB.Select(&rows, sql, append(args, limit, offset)...); err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	var total int
	h.DB.Get(&total, "SELECT COUNT(*) FROM denied_attempts d WHERE "+whereSQL, args...)

	jsonOK(w, map[string]any{"items": rows, "total": total, "limit": limit, "offset": offset})
}
