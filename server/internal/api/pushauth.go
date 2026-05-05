package api

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"proidentity/internal/pushauth"
)

var (
	pushClientMu  sync.Mutex
	pushClientKey string
	pushClient    *pushauth.Client

	pushCreateMu  sync.Mutex
	pushPendingMu sync.Mutex
	pushPending   = map[string]pendingPushAuth{}
)

type pendingPushAuth struct {
	UserID    string
	Purpose   string
	Context   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *Server) pushAuthEnabled() bool {
	var val string
	if err := s.db.Get(&val, "SELECT value FROM settings WHERE `key`='push_auth_enabled'"); err != nil {
		return false
	}
	return val == "true" || val == "1"
}

func (s *Server) pushAuthClient() *pushauth.Client {
	var apiKey string
	if err := s.db.Get(&apiKey, "SELECT value FROM settings WHERE `key`='push_auth_api_key'"); err != nil || apiKey == "" {
		return nil
	}
	pushClientMu.Lock()
	defer pushClientMu.Unlock()
	if pushClient == nil || pushClientKey != apiKey {
		pushClient = pushauth.NewClient(apiKey)
		pushClientKey = apiKey
	}
	return pushClient
}

func rememberPushAuth(requestID, userID, purpose, context string, ttlSeconds int) pendingPushAuth {
	now := time.Now()
	pending := pendingPushAuth{
		UserID:    userID,
		Purpose:   purpose,
		Context:   context,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(ttlSeconds) * time.Second),
	}
	pushPendingMu.Lock()
	pushPending[requestID] = pending
	pushPendingMu.Unlock()
	return pending
}

func forgetPushAuth(requestID string) {
	pushPendingMu.Lock()
	delete(pushPending, requestID)
	pushPendingMu.Unlock()
}

func findReusablePendingPushAuth(pc *pushauth.Client, userID, purpose, context string) (string, pendingPushAuth, bool) {
	type candidate struct {
		id      string
		pending pendingPushAuth
	}

	now := time.Now()
	var candidates []candidate
	pushPendingMu.Lock()
	for id, pending := range pushPending {
		if now.After(pending.ExpiresAt) {
			delete(pushPending, id)
			continue
		}
		if pending.UserID == userID && pending.Purpose == purpose && pending.Context == context {
			candidates = append(candidates, candidate{id: id, pending: pending})
		}
	}
	pushPendingMu.Unlock()

	for _, cand := range candidates {
		st, err := pc.PollStatus(cand.id)
		if err != nil {
			// For very recent requests, prefer local reuse over creating a
			// duplicate notification when the status endpoint is briefly slow.
			if time.Since(cand.pending.CreatedAt) < 10*time.Second {
				return cand.id, cand.pending, true
			}
			continue
		}
		switch st.Status {
		case "pending":
			return cand.id, cand.pending, true
		case "denied", "expired":
			forgetPushAuth(cand.id)
		}
	}

	return "", pendingPushAuth{}, false
}

func verifyBoundPushAuth(pc *pushauth.Client, requestID, userID, purpose string) error {
	pushPendingMu.Lock()
	pending, ok := pushPending[requestID]
	if !ok {
		pushPendingMu.Unlock()
		return fmt.Errorf("push auth request not found")
	}
	if pending.UserID != userID || pending.Purpose != purpose {
		pushPendingMu.Unlock()
		return fmt.Errorf("push auth request does not match user or purpose")
	}
	if time.Now().After(pending.ExpiresAt) {
		delete(pushPending, requestID)
		pushPendingMu.Unlock()
		return fmt.Errorf("push auth request expired")
	}
	pushPendingMu.Unlock()

	st, err := pc.PollStatus(requestID)
	if err != nil {
		return err
	}
	if st.Status != "approved" {
		return fmt.Errorf("push auth not approved")
	}

	pushPendingMu.Lock()
	delete(pushPending, requestID)
	pushPendingMu.Unlock()
	return nil
}

func lookupPendingPushAuth(requestID string) (pendingPushAuth, bool) {
	pushPendingMu.Lock()
	defer pushPendingMu.Unlock()

	pending, ok := pushPending[requestID]
	if !ok {
		return pendingPushAuth{}, false
	}
	if time.Now().After(pending.ExpiresAt) {
		delete(pushPending, requestID)
		return pendingPushAuth{}, false
	}
	return pending, true
}

// POST /api/v1/auth/push — create a push auth request for the current user
func (s *Server) handlePushAuthCreate(w http.ResponseWriter, r *http.Request) {
	if !s.pushAuthEnabled() {
		jsonError(w, http.StatusBadRequest, "push auth not enabled")
		return
	}
	pc := s.pushAuthClient()
	if pc == nil {
		jsonError(w, http.StatusInternalServerError, "push auth not configured")
		return
	}

	claims := claimsFrom(r)
	var email string
	if err := s.db.Get(&email, "SELECT email FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusInternalServerError, "user not found")
		return
	}

	var req struct {
		Context string `json:"context"`
	}
	decode(r, &req)
	if req.Context == "" {
		req.Context = "VPN connection"
	}

	clientIP := remoteAddr(r)

	pushCreateMu.Lock()
	defer pushCreateMu.Unlock()

	if requestID, pending, ok := findReusablePendingPushAuth(pc, claims.UserID, "session", req.Context); ok {
		jsonOK(w, map[string]any{
			"request_id": requestID,
			"status":     "pending",
			"expires_at": pending.ExpiresAt.Unix(),
		})
		return
	}

	ar, err := pc.CreateAuthRequest(email, "ProIdentity Access", req.Context, clientIP, 120)
	if err != nil {
		log.Printf("push auth create failed for %s: %v", email, err)
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	rememberPushAuth(ar.RequestID, claims.UserID, "session", req.Context, 120)
	jsonOK(w, map[string]any{
		"request_id": ar.RequestID,
		"status":     ar.Status,
		"expires_at": ar.ExpiresAt,
	})
}

// GET /api/v1/auth/push/{id} — poll push auth status
func (s *Server) handlePushAuthPoll(w http.ResponseWriter, r *http.Request) {
	pc := s.pushAuthClient()
	if pc == nil {
		jsonError(w, http.StatusInternalServerError, "push auth not configured")
		return
	}
	reqID := chi.URLParam(r, "id")
	claims := claimsFrom(r)
	pending, ok := lookupPendingPushAuth(reqID)
	if !ok || pending.UserID != claims.UserID || pending.Purpose != "session" {
		jsonError(w, http.StatusNotFound, "push auth request not found")
		return
	}
	st, err := pc.PollStatus(reqID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if st.Status == "denied" || st.Status == "expired" {
		forgetPushAuth(reqID)
	}
	jsonOK(w, st)
}

// GET /api/v1/auth/push-status/{id} — public poll used by login and mobile connect flows.
// The request ID must still exist in the local pending registry; final use is
// bound and consumed by verifyBoundPushAuth.
func (s *Server) handlePushAuthPollPublic(w http.ResponseWriter, r *http.Request) {
	pc := s.pushAuthClient()
	if pc == nil {
		jsonError(w, http.StatusInternalServerError, "push auth not configured")
		return
	}
	reqID := chi.URLParam(r, "id")
	if _, ok := lookupPendingPushAuth(reqID); !ok {
		jsonError(w, http.StatusNotFound, "push auth request not found")
		return
	}
	st, err := pc.PollStatus(reqID)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if st.Status == "denied" || st.Status == "expired" {
		forgetPushAuth(reqID)
	}
	jsonOK(w, map[string]string{"status": st.Status})
}

// POST /api/v1/auth/push/verify-totp — fallback: verify TOTP code via ProIdentity Cloud
func (s *Server) handlePushVerifyTOTP(w http.ResponseWriter, r *http.Request) {
	if !s.pushAuthEnabled() {
		jsonError(w, http.StatusBadRequest, "push auth not enabled")
		return
	}
	pc := s.pushAuthClient()
	if pc == nil {
		jsonError(w, http.StatusInternalServerError, "push auth not configured")
		return
	}

	claims := claimsFrom(r)
	var email string
	if err := s.db.Get(&email, "SELECT email FROM users WHERE id=?", claims.UserID); err != nil {
		jsonError(w, http.StatusInternalServerError, "user not found")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := decode(r, &req); err != nil || req.Code == "" {
		jsonError(w, http.StatusBadRequest, "code required")
		return
	}

	valid, err := pc.VerifyTOTP(email, req.Code)
	if err != nil {
		jsonError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !valid {
		jsonError(w, http.StatusUnauthorized, "invalid 2FA code")
		return
	}
	jsonOK(w, map[string]bool{"valid": true})
}
