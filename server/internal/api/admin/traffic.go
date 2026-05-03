package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

// TrafficHandler exposes /admin/traffic.
type TrafficHandler struct{ DB *sqlx.DB }

// TopRow is one (group, bytes_tx, bytes_rx) entry returned by the aggregation.
type TopRow struct {
	Key       string `db:"k"        json:"key"`
	Label     string `db:"label"    json:"label"`
	BytesTX   int64  `db:"bytes_tx" json:"bytes_tx"`
	BytesRX   int64  `db:"bytes_rx" json:"bytes_rx"`
	Conns     int64  `db:"conns"    json:"conns"`
}

// Top returns aggregated traffic for the requested grouping.
//
// GET /admin/traffic/top
//   ?by=user|resource|user_resource|destination|port  (required)
//   &since=RFC3339  (optional, default: 24h ago)
//   &user_id=...    (optional filter)
//   &resource_id=...(optional filter)
//   &server_id=...  (optional filter)
//   &limit=10       (optional, default 10, max 200)
func (h *TrafficHandler) Top(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	by := q.Get("by")
	if by == "" {
		jsonError(w, 400, "missing 'by' param (user|resource|user_resource|destination|port)")
		return
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 10
	}
	since := time.Now().Add(-24 * time.Hour)
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}

	where := "ts >= ?"
	args := []any{since}
	if v := q.Get("user_id"); v != "" {
		where += " AND user_id = ?"
		args = append(args, v)
	}
	if v := q.Get("resource_id"); v != "" {
		where += " AND resource_id = ?"
		args = append(args, v)
	}
	if v := q.Get("server_id"); v != "" {
		where += " AND server_id = ?"
		args = append(args, v)
	}

	var sql string
	switch by {
	case "user":
		sql = `
			SELECT IFNULL(t.user_id, '')                              AS k,
			       IFNULL(u.username, '(unknown)')                    AS label,
			       SUM(t.bytes_tx)                                    AS bytes_tx,
			       SUM(t.bytes_rx)                                    AS bytes_rx,
			       COUNT(*)                                           AS conns
			FROM traffic_flows t
			LEFT JOIN users u ON u.id COLLATE utf8mb4_unicode_ci = t.user_id COLLATE utf8mb4_unicode_ci
			WHERE ` + where + `
			GROUP BY t.user_id
			ORDER BY (SUM(t.bytes_tx) + SUM(t.bytes_rx)) DESC
			LIMIT ?`
	case "resource":
		sql = `
			SELECT IFNULL(t.resource_id, '')                          AS k,
			       IFNULL(r.name, CONCAT('(unmatched dst ', t.dst_ip, ')')) AS label,
			       SUM(t.bytes_tx)                                    AS bytes_tx,
			       SUM(t.bytes_rx)                                    AS bytes_rx,
			       COUNT(*)                                           AS conns
			FROM traffic_flows t
			LEFT JOIN resources r ON r.id COLLATE utf8mb4_unicode_ci = t.resource_id COLLATE utf8mb4_unicode_ci
			WHERE ` + where + `
			GROUP BY t.resource_id, t.dst_ip
			ORDER BY (SUM(t.bytes_tx) + SUM(t.bytes_rx)) DESC
			LIMIT ?`
	case "user_resource":
		sql = `
			SELECT CONCAT(t.user_id, ':', t.server_id, ':', t.resource_id) AS k,
			       CONCAT(IFNULL(u.username, '(unknown)'), ' -> ', IFNULL(r.name, '(unknown resource)')) AS label,
			       SUM(t.bytes_tx)                                    AS bytes_tx,
			       SUM(t.bytes_rx)                                    AS bytes_rx,
			       COUNT(*)                                           AS conns
			FROM traffic_flows t
			LEFT JOIN users u ON u.id COLLATE utf8mb4_unicode_ci = t.user_id COLLATE utf8mb4_unicode_ci
			LEFT JOIN resources r ON r.id COLLATE utf8mb4_unicode_ci = t.resource_id COLLATE utf8mb4_unicode_ci
			WHERE ` + where + `
			  AND t.user_id IS NOT NULL AND t.user_id <> ''
			  AND t.server_id IS NOT NULL AND t.server_id <> ''
			  AND t.resource_id IS NOT NULL AND t.resource_id <> ''
			GROUP BY t.user_id, t.server_id, t.resource_id
			ORDER BY (SUM(t.bytes_tx) + SUM(t.bytes_rx)) DESC
			LIMIT ?`
	case "destination":
		sql = `
			SELECT t.dst_ip                AS k,
			       t.dst_ip                AS label,
			       SUM(t.bytes_tx)         AS bytes_tx,
			       SUM(t.bytes_rx)         AS bytes_rx,
			       COUNT(*)                AS conns
			FROM traffic_flows t
			WHERE ` + where + `
			GROUP BY t.dst_ip
			ORDER BY (SUM(t.bytes_tx) + SUM(t.bytes_rx)) DESC
			LIMIT ?`
	case "port":
		sql = `
			SELECT CONCAT(t.proto, '/', IFNULL(t.dst_port, 0))  AS k,
			       CONCAT(UPPER(t.proto), ' ',
			              IFNULL(CAST(t.dst_port AS CHAR), '—')) AS label,
			       SUM(t.bytes_tx)                              AS bytes_tx,
			       SUM(t.bytes_rx)                              AS bytes_rx,
			       COUNT(*)                                     AS conns
			FROM traffic_flows t
			WHERE ` + where + `
			GROUP BY t.proto, t.dst_port
			ORDER BY (SUM(t.bytes_tx) + SUM(t.bytes_rx)) DESC
			LIMIT ?`
	default:
		jsonError(w, 400, "invalid 'by' param")
		return
	}

	rows := []TopRow{}
	if err := h.DB.Select(&rows, sql, append(args, limit)...); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, rows)
}

// SummaryRow is total bytes/connections for a single subject (the filtered set).
type SummaryRow struct {
	BytesTX int64 `db:"bytes_tx" json:"bytes_tx"`
	BytesRX int64 `db:"bytes_rx" json:"bytes_rx"`
	Conns   int64 `db:"conns"    json:"conns"`
}

// Summary returns a single aggregate over the filtered window.
// Same filters as Top.
func (h *TrafficHandler) Summary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	since := time.Now().Add(-24 * time.Hour)
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	where := "ts >= ?"
	args := []any{since}
	if v := q.Get("user_id"); v != "" {
		where += " AND user_id = ?"
		args = append(args, v)
	}
	if v := q.Get("resource_id"); v != "" {
		where += " AND resource_id = ?"
		args = append(args, v)
	}
	if v := q.Get("server_id"); v != "" {
		where += " AND server_id = ?"
		args = append(args, v)
	}
	var row SummaryRow
	err := h.DB.Get(&row, `
		SELECT IFNULL(SUM(bytes_tx), 0) AS bytes_tx,
		       IFNULL(SUM(bytes_rx), 0) AS bytes_rx,
		       COUNT(*) AS conns
		FROM traffic_flows
		WHERE `+where, args...)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	jsonOK(w, row)
}
