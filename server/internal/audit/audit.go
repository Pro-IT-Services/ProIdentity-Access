// Package audit records who-did-what to a database table.
package audit

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Entry is one audited event.
type Entry struct {
	ActorUserID   string
	ActorUsername string
	Method        string
	Path          string
	Action        string // optional structured tag, e.g. "auth.login"
	TargetType    string
	TargetID      string
	TargetLabel   string
	StatusCode    int
	Success       bool
	ErrorMessage  string
	IP            string
	UserAgent     string
	Detail        any // marshaled to JSON
}

// Recorder writes audit entries.
type Recorder struct{ db *sqlx.DB }

func New(db *sqlx.DB) *Recorder { return &Recorder{db: db} }

// Log persists one entry. Errors are logged but not returned — auditing
// must never break a user-facing request.
func (r *Recorder) Log(e Entry) {
	if r == nil || r.db == nil {
		return
	}
	var detailJSON any
	if e.Detail != nil {
		b, err := json.Marshal(e.Detail)
		if err == nil {
			detailJSON = string(b)
		}
	}
	_, err := r.db.Exec(`
		INSERT INTO audit_logs
		  (id, actor_user_id, actor_username, method, path, action, target_type, target_id, target_label,
		   status_code, success, error_message, ip, user_agent, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		nullIfEmpty(e.ActorUserID),
		nullIfEmpty(e.ActorUsername),
		e.Method, e.Path,
		nullIfEmpty(e.Action), nullIfEmpty(e.TargetType), nullIfEmpty(e.TargetID), nullIfEmpty(e.TargetLabel),
		e.StatusCode, boolToInt(e.Success), nullIfEmpty(e.ErrorMessage),
		nullIfEmpty(e.IP), nullIfEmpty(truncate(e.UserAgent, 255)),
		detailJSON,
	)
	if err != nil {
		log.Printf("audit: insert failed: %v", err)
	}
}

func nullIfEmpty(s string) any { if s == "" { return nil }; return s }
func boolToInt(b bool) int     { if b { return 1 }; return 0 }
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// IPFromRequest extracts the client IP, honoring X-Forwarded-For if present.
func IPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the original client.
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if ra := r.RemoteAddr; ra != "" {
		// Strip ":port"
		if i := strings.LastIndex(ra, ":"); i > 0 {
			return ra[:i]
		}
		return ra
	}
	return ""
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// recordKey is a context key carrying a per-request override of the audit fields.
type recordKey struct{}

// PerRequest lets a handler stash extra audit fields (target id, label, action, detail)
// before responding. The middleware reads this back when writing the entry.
type PerRequest struct {
	Action      string
	TargetType  string
	TargetID    string
	TargetLabel string
	Detail      any
}

// Annotate stores per-request audit fields on the context. Pass the *Request — the value
// is mutated in place via a pointer stored in the context.
func Annotate(r *http.Request, fn func(*PerRequest)) {
	pr, _ := r.Context().Value(recordKey{}).(*PerRequest)
	if pr == nil {
		return
	}
	fn(pr)
}

// WithPerRequest attaches an empty PerRequest to the context (used by the middleware).
func WithPerRequest(ctx context.Context) (context.Context, *PerRequest) {
	pr := &PerRequest{}
	return context.WithValue(ctx, recordKey{}, pr), pr
}
