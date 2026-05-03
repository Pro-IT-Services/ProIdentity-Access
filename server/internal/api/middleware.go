package api

import (
	"context"
	"net/http"
	"strings"

	"proidentity/internal/audit"
	"proidentity/internal/auth"
)

type ctxKey string

const (
	claimsKey        = auth.ClaimsCtxKey
	permsKey  ctxKey = "perms"
)

// Authenticate is a middleware that validates the Bearer JWT and loads the
// user's effective permissions into the request context.
func (s *Server) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			jsonError(w, http.StatusUnauthorized, "missing token")
			return
		}
		claims, err := auth.ParseToken(strings.TrimPrefix(hdr, "Bearer "), s.cfg.Auth.JWTSecret)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		var current struct {
			Username string `db:"username"`
			IsAdmin  bool   `db:"is_admin"`
			IsActive bool   `db:"is_active"`
		}
		if err := s.db.Get(&current, "SELECT username, is_admin, is_active FROM users WHERE id=?", claims.UserID); err != nil {
			jsonError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		if !current.IsActive {
			jsonError(w, http.StatusUnauthorized, "user disabled")
			return
		}
		claims.Username = current.Username
		claims.IsAdmin = current.IsAdmin
		perms, _ := auth.LoadUserPermissions(s.db, claims.UserID, claims.IsAdmin)
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = context.WithValue(ctx, permsKey, perms)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin rejects non-admin users.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(claimsKey).(*auth.Claims)
		if !ok || !claims.IsAdmin {
			jsonError(w, http.StatusForbidden, "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func claimsFrom(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(claimsKey).(*auth.Claims)
	return c
}

func permsFrom(r *http.Request) []auth.Perm {
	p, _ := r.Context().Value(permsKey).([]auth.Perm)
	return p
}

func hasPerm(r *http.Request, p auth.Perm) bool {
	for _, x := range permsFrom(r) {
		if x == p {
			return true
		}
	}
	return false
}

// RequirePerm returns a middleware that 403s if the caller lacks the named
// permission. is_admin users automatically hold every permission (loaded by
// Authenticate), so this also covers the legacy "admin" gate.
func RequirePerm(p auth.Perm) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !hasPerm(r, p) {
				jsonError(w, http.StatusForbidden, "missing permission: "+string(p))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// auditMiddleware records every authenticated mutating request (POST, PUT, PATCH, DELETE)
// to the audit log. Reads (GET, HEAD, OPTIONS) are skipped to keep noise down.
func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only audit mutating methods.
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		ctx, pr := audit.WithPerRequest(r.Context())
		sw := &auditStatusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r.WithContext(ctx))

		claims := claimsFrom(r)
		entry := audit.Entry{
			Method:      r.Method,
			Path:        r.URL.Path,
			Action:      pr.Action,
			TargetType:  pr.TargetType,
			TargetID:    pr.TargetID,
			TargetLabel: pr.TargetLabel,
			StatusCode:  sw.status,
			Success:     sw.status >= 200 && sw.status < 400,
			IP:          remoteAddr(r),
			UserAgent:   r.UserAgent(),
			Detail:      pr.Detail,
		}
		if claims != nil {
			entry.ActorUserID = claims.UserID
			entry.ActorUsername = claims.Username
		}
		s.audit.Log(entry)
	})
}

type auditStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
