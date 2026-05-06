package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type VPNEventHandler struct{ DB *sqlx.DB }

type VPNEventRow struct {
	ID         string    `db:"id" json:"id"`
	EventType  string    `db:"event_type" json:"event_type"`
	Reason     *string   `db:"reason" json:"reason"`
	UserID     *string   `db:"user_id" json:"user_id"`
	Username   *string   `db:"username" json:"username"`
	Email      *string   `db:"email" json:"email"`
	SessionID  string    `db:"session_id" json:"session_id"`
	ServerID   *string   `db:"server_id" json:"server_id"`
	ServerName *string   `db:"server_name" json:"server_name"`
	AssignedIP string    `db:"assigned_ip" json:"assigned_ip"`
	SourceIP   *string   `db:"source_ip" json:"source_ip"`
	DeviceID   *string   `db:"device_id" json:"device_id"`
	DeviceName *string   `db:"device_name" json:"device_name"`
	UserAgent  *string   `db:"user_agent" json:"user_agent"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

func (h *VPNEventHandler) List(w http.ResponseWriter, r *http.Request) {
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
	if v := strings.TrimSpace(q.Get("user_id")); v != "" {
		where = append(where, "user_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("server_id")); v != "" {
		where = append(where, "server_id = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("event")); v == "connected" || v == "disconnected" {
		where = append(where, "event_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("device")); v != "" {
		where = append(where, "(device_name LIKE ? OR device_id = ?)")
		args = append(args, "%"+v+"%", v)
	}
	if v := strings.TrimSpace(q.Get("source_ip")); v != "" {
		where = append(where, "source_ip = ?")
		args = append(args, v)
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			where = append(where, "created_at >= ?")
			args = append(args, t)
		}
	}

	whereSQL := strings.Join(where, " AND ")
	rows := []VPNEventRow{}
	sql := "SELECT * FROM vpn_connection_events WHERE " + whereSQL + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	if err := h.DB.Select(&rows, sql, append(args, limit, offset)...); err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	var total int
	_ = h.DB.Get(&total, "SELECT COUNT(*) FROM vpn_connection_events WHERE "+whereSQL, args...)
	jsonOK(w, map[string]any{
		"items":  rows,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
