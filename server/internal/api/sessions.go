package api

import (
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"
	"proidentity/internal/auth"
)

// GET /api/v1/servers — lists servers available to the authenticated user
func (s *Server) handleListUserServers(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	servers, err := s.registry.UserServers(claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, servers)
}

// POST /api/v1/sessions
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)

	var req struct {
		ServerID        string `json:"server_id"`
		ClientPublicKey string `json:"client_public_key"`
		TOTPCode        string `json:"totp_code"`
		PushAuthID      string `json:"push_auth_request_id"`
	}
	if err := decode(r, &req); err != nil || req.ClientPublicKey == "" {
		jsonError(w, http.StatusBadRequest, "server_id and client_public_key required")
		return
	}
	if req.ServerID == "" {
		jsonError(w, http.StatusBadRequest, "server_id required")
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.ClientPublicKey)
	if err != nil || len(pub) != 32 {
		jsonError(w, http.StatusBadRequest, "client_public_key must be a base64 WireGuard public key")
		return
	}

	if !claims.IsAdmin {
		var allowed int
		if err := s.db.Get(&allowed, `
			SELECT COUNT(*)
			FROM user_server_access usa
			JOIN wg_servers s ON s.id = usa.server_id
			WHERE usa.user_id = ? AND usa.server_id = ? AND s.is_active = 1`,
			claims.UserID, req.ServerID,
		); err != nil {
			jsonError(w, http.StatusInternalServerError, "server access check failed")
			return
		}
		if allowed == 0 {
			jsonError(w, http.StatusForbidden, "server access denied")
			return
		}
	}

	var totpEnabled bool
	var totpSecret *string
	if err := s.db.QueryRow("SELECT totp_enabled, totp_secret FROM users WHERE id=?", claims.UserID).
		Scan(&totpEnabled, &totpSecret); err != nil {
		jsonError(w, http.StatusInternalServerError, "user not found")
		return
	}
	if totpEnabled {
		pushEnabled := s.pushAuthEnabled()

		if req.TOTPCode == "" && req.PushAuthID == "" {
			jsonOK(w, map[string]any{"require_totp": true, "push_auth_enabled": pushEnabled})
			return
		}

		if req.PushAuthID != "" {
			pc := s.pushAuthClient()
			if pc == nil {
				jsonError(w, http.StatusInternalServerError, "push auth not configured")
				return
			}
			if err := verifyBoundPushAuth(pc, req.PushAuthID, claims.UserID, "session"); err != nil {
				jsonError(w, http.StatusUnauthorized, "push auth not approved")
				return
			}
		} else if req.TOTPCode != "" {
			if totpSecret == nil || !auth.ValidateTOTP(*totpSecret, req.TOTPCode) {
				jsonError(w, http.StatusUnauthorized, "invalid 2FA code")
				return
			}
		}
	}

	sess, wgConfig, err := s.sessions.CreateSession(claims.UserID, req.ServerID, req.ClientPublicKey)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	vpnName := s.sessions.Setting("vpn_name")
	if vpnName == "" {
		vpnName = "Managed VPN"
	}
	jsonOK(w, map[string]any{
		"session_id":  sess.ID,
		"assigned_ip": sess.AssignedIP,
		"server_id":   sess.ServerID,
		"wg_config":   wgConfig,
		"vpn_name":    vpnName,
	})
}

// POST /api/v1/sessions/{id}/keepalive
func (s *Server) handleKeepalive(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")

	// Verify session belongs to this user
	sess, err := s.sessions.GetSession(id)
	if err != nil || sess.UserID != claims.UserID {
		jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	if err := s.sessions.Keepalive(id); err != nil {
		jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/sessions/{id}
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")

	sess, err := s.sessions.GetSession(id)
	if err != nil || (sess.UserID != claims.UserID && !claims.IsAdmin) {
		jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	if err := s.sessions.Terminate(id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// GET /api/v1/sessions/mine
func (s *Server) handleMySessions(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	rows, err := s.db.Queryx(`
		SELECT s.id, s.server_id, s.assigned_ip, s.created_at, s.last_keepalive
		FROM sessions s WHERE s.user_id=?
		ORDER BY s.created_at DESC`, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	sessions := []map[string]any{}
	for rows.Next() {
		m := map[string]any{}
		rows.MapScan(m)
		for k, v := range m {
			if b, ok := v.([]byte); ok {
				m[k] = string(b)
			}
		}
		sessions = append(sessions, m)
	}
	jsonOK(w, sessions)
}
