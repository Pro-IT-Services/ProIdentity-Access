package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// AuditHandler exposes /admin/audit.
type AuditHandler struct{ DB *sqlx.DB }

// AuditRow is one row of the audit log.
type AuditRow struct {
	ID            string    `db:"id"             json:"id"`
	TS            time.Time `db:"ts"             json:"ts"`
	ActorUserID   *string   `db:"actor_user_id"  json:"actor_user_id"`
	ActorUsername *string   `db:"actor_username" json:"actor_username"`
	Method        string    `db:"method"         json:"method"`
	Path          string    `db:"path"           json:"path"`
	Action        *string   `db:"action"         json:"action"`
	TargetType    *string   `db:"target_type"    json:"target_type"`
	TargetID      *string   `db:"target_id"      json:"target_id"`
	TargetLabel   *string   `db:"target_label"   json:"target_label"`
	StatusCode    int       `db:"status_code"    json:"status_code"`
	Success       int       `db:"success"        json:"success"`
	ErrorMessage  *string   `db:"error_message"  json:"error_message"`
	IP            *string   `db:"ip"             json:"ip"`
	UserAgent     *string   `db:"user_agent"     json:"user_agent"`
	Detail        *string   `db:"detail"         json:"detail"`
}

// List GET /admin/audit?limit=&offset=&actor=&action=&target_type=&target_id=&since=
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
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
	if v := q.Get("actor"); v != "" {
		where = append(where, "(actor_username LIKE ? OR actor_user_id = ?)")
		args = append(args, "%"+v+"%", v)
	}
	if v := q.Get("action"); v != "" {
		where = append(where, "(action LIKE ? OR path LIKE ?)")
		args = append(args, "%"+v+"%", "%"+v+"%")
	}
	if v := q.Get("target_type"); v != "" {
		where = append(where, "target_type = ?")
		args = append(args, v)
	}
	if v := q.Get("target_id"); v != "" {
		where = append(where, "target_id = ?")
		args = append(args, v)
	}
	if v := q.Get("since"); v != "" {
		// since is RFC3339; if invalid, ignore.
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			where = append(where, "ts >= ?")
			args = append(args, t)
		}
	}

	sql := "SELECT * FROM audit_logs WHERE "
	for i, c := range where {
		if i > 0 {
			sql += " AND "
		}
		sql += c
	}
	sql += " ORDER BY ts DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows := []AuditRow{}
	if err := h.DB.Select(&rows, sql, args...); err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	// Count for paging.
	countSQL := "SELECT COUNT(*) FROM audit_logs WHERE "
	for i, c := range where {
		if i > 0 {
			countSQL += " AND "
		}
		countSQL += c
	}
	var total int
	h.DB.Get(&total, countSQL, args[:len(args)-2]...)

	jsonOK(w, map[string]any{
		"items":  rows,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
