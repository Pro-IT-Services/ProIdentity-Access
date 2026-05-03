package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"proidentity/internal/audit"
	"proidentity/internal/auth"
	"proidentity/internal/model"
)

// WebAuthn pending sessions (stored in memory, expire after 5 min)
type pendingWA struct {
	data      []byte
	userID    string
	expiresAt time.Time
}

var (
	waMu      sync.Mutex
	waPending = map[string]*pendingWA{}
)

// POST /api/v1/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		TOTPCode   string `json:"totp_code"`
		PushAuthID string `json:"push_auth_request_id"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	logFail := func(reason string, status int) {
		s.audit.Log(audit.Entry{
			ActorUsername: req.Username,
			Method:        r.Method,
			Path:          r.URL.Path,
			Action:        "auth.login.fail",
			TargetType:    "user",
			TargetLabel:   req.Username,
			StatusCode:    status,
			Success:       false,
			ErrorMessage:  reason,
			IP:            remoteAddr(r),
			UserAgent:     r.UserAgent(),
		})
	}

	var user model.User
	if err := s.db.Get(&user,
		"SELECT * FROM users WHERE username=? AND is_active=1", req.Username); err != nil {
		logFail("unknown user", http.StatusUnauthorized)
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		logFail("bad password", http.StatusUnauthorized)
		jsonError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.TOTPEnabled {
		pushEnabled := s.pushAuthEnabled()

		// No credentials provided — tell client what's available.
		if req.TOTPCode == "" && req.PushAuthID == "" {
			resp := map[string]any{"require_totp": true, "push_auth_enabled": pushEnabled}
			if pushEnabled {
				pc := s.pushAuthClient()
				if pc != nil {
					clientIP := remoteAddr(r)
					ar, err := pc.CreateAuthRequest(user.Email, "ProIdentity Access", "Admin login", clientIP, 120)
					if err == nil {
						rememberPushAuth(ar.RequestID, user.ID, "login", 120)
						resp["push_request_id"] = ar.RequestID
					} else {
						log.Printf("push auth create failed for login %s: %v", user.Email, err)
					}
				}
			}
			jsonOK(w, resp)
			return
		}

		// Push auth path — verify the push request is approved.
		if req.PushAuthID != "" {
			pc := s.pushAuthClient()
			if pc == nil {
				jsonError(w, http.StatusInternalServerError, "push auth not configured")
				return
			}
			if err := verifyBoundPushAuth(pc, req.PushAuthID, user.ID, "login"); err != nil {
				logFail("push auth not approved", http.StatusUnauthorized)
				jsonError(w, http.StatusUnauthorized, "push auth not approved")
				return
			}
		} else if req.TOTPCode != "" {
			// TOTP code path — verify locally.
			if user.TOTPSecret == nil || !auth.ValidateTOTP(*user.TOTPSecret, req.TOTPCode) {
				logFail("bad totp", http.StatusUnauthorized)
				jsonError(w, http.StatusUnauthorized, "invalid 2FA code")
				return
			}
		}
	}

	token, err := auth.IssueToken(user.ID, user.Username, user.IsAdmin, s.cfg.Auth.JWTSecret, 24*time.Hour)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token error")
		return
	}

	// Link installation to user if this request came from a registered device
	if deviceID := r.Header.Get("X-Device-ID"); deviceID != "" {
		s.db.Exec("UPDATE installations SET user_id=? WHERE id=?", user.ID, deviceID)
	}

	s.audit.Log(audit.Entry{
		ActorUserID: user.ID, ActorUsername: user.Username,
		Method: r.Method, Path: r.URL.Path,
		Action:     "auth.login",
		TargetType: "user", TargetID: user.ID, TargetLabel: user.Username,
		StatusCode: http.StatusOK, Success: true,
		IP: remoteAddr(r), UserAgent: r.UserAgent(),
	})

	jsonOK(w, map[string]any{
		"token":        token,
		"user_id":      user.ID,
		"username":     user.Username,
		"is_admin":     user.IsAdmin,
		"totp_enabled": user.TOTPEnabled,
	})
}

// GET /api/v1/auth/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	jsonOK(w, map[string]any{
		"id":           user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"first_name":   user.FirstName,
		"last_name":    user.LastName,
		"is_admin":     user.IsAdmin,
		"is_active":    user.IsActive,
		"totp_enabled": user.TOTPEnabled,
		"created_at":   user.CreatedAt,
		"permissions":  permsFrom(r),
	})
}

// GET /api/v1/auth/permissions — full catalog of permission keys.
func (s *Server) handlePermCatalog(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, auth.PermCatalog())
}

// POST /api/v1/auth/change-password
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil || req.NewPassword == "" {
		jsonError(w, http.StatusBadRequest, "current_password and new_password required")
		return
	}
	claims := claimsFrom(r)
	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		jsonError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "hash error")
		return
	}
	if _, err := s.db.Exec("UPDATE users SET password_hash=? WHERE id=?", hash, claims.UserID); err != nil {
		jsonError(w, http.StatusInternalServerError, "update failed")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// POST /api/v1/auth/totp/setup — initiate TOTP setup for authenticated user
func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	secret, uri, err := auth.GenerateTOTP(claims.Username, "ProIdentity Access")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "generate totp failed")
		return
	}
	// Store secret temporarily (user must confirm before enabling)
	_, err = s.db.Exec("UPDATE users SET totp_secret=? WHERE id=?", secret, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "store secret failed")
		return
	}
	jsonOK(w, map[string]string{"secret": secret, "uri": uri})
}

// POST /api/v1/auth/totp/confirm — verify code and enable TOTP
func (s *Server) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request")
		return
	}
	claims := claimsFrom(r)
	var secret *string
	if err := s.db.Get(&secret, "SELECT totp_secret FROM users WHERE id=?", claims.UserID); err != nil || secret == nil {
		jsonError(w, http.StatusBadRequest, "no pending TOTP setup")
		return
	}
	if !auth.ValidateTOTP(*secret, req.Code) {
		jsonError(w, http.StatusBadRequest, "invalid code")
		return
	}
	_, err := s.db.Exec("UPDATE users SET totp_enabled=1 WHERE id=?", claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "enable totp failed")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// POST /api/v1/auth/totp/disable
func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request")
		return
	}
	claims := claimsFrom(r)
	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	if user.TOTPEnabled && (user.TOTPSecret == nil || !auth.ValidateTOTP(*user.TOTPSecret, req.Code)) {
		jsonError(w, http.StatusUnauthorized, "invalid code")
		return
	}
	_, err := s.db.Exec("UPDATE users SET totp_enabled=0, totp_secret=NULL WHERE id=?", claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "disable totp failed")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// POST /api/v1/auth/passkey/register/begin
func (s *Server) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}

	var passkeys []*model.Passkey
	s.db.Select(&passkeys, "SELECT * FROM passkeys WHERE user_id=?", user.ID)

	waUser := &auth.WebAuthnUser{User: &user, Passkeys: passkeys}
	creation, sessionData, err := s.wa.BeginRegistration(waUser)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("begin registration: %v", err))
		return
	}

	sdBytes, _ := auth.MarshalSession(sessionData)
	waMu.Lock()
	waPending[user.ID+"_reg"] = &pendingWA{data: sdBytes, userID: user.ID, expiresAt: time.Now().Add(5 * time.Minute)}
	waMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(creation)
}

// POST /api/v1/auth/passkey/register/finish
func (s *Server) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	var name string
	if v := r.URL.Query().Get("name"); v != "" {
		name = v
	} else {
		name = "Passkey"
	}

	claims := claimsFrom(r)
	waMu.Lock()
	pending := waPending[claims.UserID+"_reg"]
	waMu.Unlock()
	if pending == nil || time.Now().After(pending.expiresAt) {
		jsonError(w, http.StatusBadRequest, "no pending registration")
		return
	}

	sd, err := auth.UnmarshalSession(pending.data)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "session data error")
		return
	}

	var user model.User
	s.db.Get(&user, "SELECT * FROM users WHERE id=?", claims.UserID)
	var passkeys []*model.Passkey
	s.db.Select(&passkeys, "SELECT * FROM passkeys WHERE user_id=?", user.ID)
	waUser := &auth.WebAuthnUser{User: &user, Passkeys: passkeys}

	cred, err := s.wa.FinishRegistration(waUser, *sd, r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("finish registration: %v", err))
		return
	}

	id := newUUID()
	_, err = s.db.Exec(`
		INSERT INTO passkeys (id, user_id, name, credential_id, public_key, sign_count, aaguid)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, user.ID, name, cred.ID, cred.PublicKey, cred.Authenticator.SignCount, fmt.Sprintf("%x", cred.Authenticator.AAGUID),
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "save passkey failed")
		return
	}

	waMu.Lock()
	delete(waPending, claims.UserID+"_reg")
	waMu.Unlock()

	jsonOK(w, map[string]string{"id": id, "name": name})
}

// POST /api/v1/auth/passkey/login/begin  (unauthenticated)
func (s *Server) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request")
		return
	}

	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE username=? AND is_active=1", req.Username); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	var passkeys []*model.Passkey
	s.db.Select(&passkeys, "SELECT * FROM passkeys WHERE user_id=?", user.ID)
	if len(passkeys) == 0 {
		jsonError(w, http.StatusBadRequest, "no passkeys registered")
		return
	}

	waUser := &auth.WebAuthnUser{User: &user, Passkeys: passkeys}
	assertion, sessionData, err := s.wa.BeginLogin(waUser)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("begin login: %v", err))
		return
	}

	sdBytes, _ := auth.MarshalSession(sessionData)
	waMu.Lock()
	waPending[user.ID+"_login"] = &pendingWA{data: sdBytes, userID: user.ID, expiresAt: time.Now().Add(5 * time.Minute)}
	waMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assertion)
}

// POST /api/v1/auth/passkey/login/finish  (unauthenticated)
func (s *Server) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	username := r.URL.Query().Get("username")

	var user model.User
	if err := s.db.Get(&user, "SELECT * FROM users WHERE username=? AND is_active=1", username); err != nil {
		jsonError(w, http.StatusNotFound, "user not found")
		return
	}
	_ = req

	waMu.Lock()
	pending := waPending[user.ID+"_login"]
	waMu.Unlock()
	if pending == nil || time.Now().After(pending.expiresAt) {
		jsonError(w, http.StatusBadRequest, "no pending login")
		return
	}

	sd, err := auth.UnmarshalSession(pending.data)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "session data error")
		return
	}

	var passkeys []*model.Passkey
	s.db.Select(&passkeys, "SELECT * FROM passkeys WHERE user_id=?", user.ID)
	waUser := &auth.WebAuthnUser{User: &user, Passkeys: passkeys}

	cred, err := s.wa.FinishLogin(waUser, *sd, r)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, fmt.Sprintf("passkey auth failed: %v", err))
		return
	}

	// Update sign count
	s.db.Exec("UPDATE passkeys SET sign_count=? WHERE credential_id=?", cred.Authenticator.SignCount, cred.ID)

	waMu.Lock()
	delete(waPending, user.ID+"_login")
	waMu.Unlock()

	token, err := auth.IssueToken(user.ID, user.Username, user.IsAdmin, s.cfg.Auth.JWTSecret, 24*time.Hour)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "token error")
		return
	}
	jsonOK(w, map[string]any{
		"token":        token,
		"user_id":      user.ID,
		"username":     user.Username,
		"is_admin":     user.IsAdmin,
		"totp_enabled": user.TOTPEnabled,
	})
}

// GET /api/v1/auth/passkeys — list user's passkeys
func (s *Server) handleListPasskeys(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	passkeys := []model.Passkey{}
	s.db.Select(&passkeys, "SELECT id, user_id, name, created_at FROM passkeys WHERE user_id=?", claims.UserID)
	jsonOK(w, passkeys)
}

// DELETE /api/v1/auth/passkeys/{id}
func (s *Server) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")
	s.db.Exec("DELETE FROM passkeys WHERE id=? AND user_id=?", id, claims.UserID)
	jsonOK(w, map[string]bool{"ok": true})
}
