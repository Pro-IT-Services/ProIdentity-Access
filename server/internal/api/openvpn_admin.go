package api

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	ovpn "proidentity/internal/openvpn"
)

// decodeOVPNConfig accepts the uploaded config as base64 (binary-safe from a
// file picker) and returns the raw .ovpn text.
func decodeOVPNConfig(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", false
	}
	if len(raw) == 0 || len(raw) > maxOVPNConfigBytes {
		return "", false
	}
	return string(raw), true
}

func normalizeDevType(v string) string {
	if strings.ToLower(strings.TrimSpace(v)) == "tap" {
		return "tap"
	}
	return "tun"
}

// ovpnNull maps an empty string to a SQL NULL.
func ovpnNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GET /api/v1/admin/openvpn/profiles
func (s *Server) handleAdminListOpenVPNProfiles(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID            string  `db:"id"              json:"id"`
		Name          string  `db:"name"            json:"name"`
		AutofillName  *string `db:"autofill_name"   json:"autofill_name,omitempty"`
		Description   *string `db:"description"     json:"description,omitempty"`
		RequiresTOTP  bool    `db:"requires_totp"   json:"requires_totp"`
		AuthUserPass  bool    `db:"auth_user_pass"  json:"auth_user_pass"`
		DevType       string  `db:"dev_type"        json:"dev_type"`
		AllowCustomIP bool    `db:"allow_custom_ip" json:"allow_custom_ip"`
		AssignedCount int     `db:"assigned_count"  json:"assigned_count"`
		CreatedAt     string  `db:"created_at"      json:"created_at"`
	}
	rows := []row{}
	err := s.db.Select(&rows, `
		SELECT p.id, p.name, p.autofill_name, p.description, p.requires_totp, p.auth_user_pass,
		       p.dev_type, p.allow_custom_ip, p.created_at,
		       (SELECT COUNT(*) FROM openvpn_profile_assignments a WHERE a.profile_id = p.id) AS assigned_count
		FROM openvpn_profiles p
		ORDER BY p.name`)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, rows)
}

// POST /api/v1/admin/openvpn/profiles
func (s *Server) handleAdminCreateOpenVPNProfile(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	var req struct {
		Name          string `json:"name"`
		AutofillName  string `json:"autofill_name"` // password-manager search text; empty = name
		Description   string `json:"description"`
		Config        string `json:"config"` // base64 of raw .ovpn
		RequiresTOTP  bool   `json:"requires_totp"`
		AllowCustomIP bool   `json:"allow_custom_ip"`
		DevType       string `json:"dev_type"` // optional override of auto-detected type
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxOVPNNameBytes {
		jsonError(w, http.StatusBadRequest, "name must be 1-255 characters")
		return
	}
	req.AutofillName = strings.TrimSpace(req.AutofillName)
	if len(req.AutofillName) > maxOVPNNameBytes {
		jsonError(w, http.StatusBadRequest, "autofill name must be at most 255 characters")
		return
	}
	raw, ok := decodeOVPNConfig(req.Config)
	if !ok {
		jsonError(w, http.StatusBadRequest, "config must be base64 of a .ovpn file (max 512 KiB)")
		return
	}

	meta := ovpn.Parse(raw)
	devType := meta.DevType
	if req.DevType != "" {
		devType = normalizeDevType(req.DevType)
	}

	id := newUUID()
	blob, err := s.encryptProfile(id, raw)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "profile encrypt failed")
		return
	}

	_, err = s.db.Exec(`
		INSERT INTO openvpn_profiles
			(id, name, autofill_name, description, config_encrypted, requires_totp, auth_user_pass, dev_type, allow_custom_ip, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, ovpnNull(req.AutofillName), ovpnNull(req.Description), blob,
		req.RequiresTOTP, meta.AuthUserPass, devType, req.AllowCustomIP, claims.UserID,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{
		"id":             id,
		"dev_type":       devType,
		"auth_user_pass": meta.AuthUserPass,
	})
}

// PUT /api/v1/admin/openvpn/profiles/{id}
func (s *Server) handleAdminUpdateOpenVPNProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name          string `json:"name"`
		AutofillName  string `json:"autofill_name"` // password-manager search text; empty = name
		Description   string `json:"description"`
		RequiresTOTP  bool   `json:"requires_totp"`
		AllowCustomIP bool   `json:"allow_custom_ip"`
		DevType       string `json:"dev_type"`
		Config        string `json:"config"` // optional base64 replacement
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxOVPNNameBytes {
		jsonError(w, http.StatusBadRequest, "name must be 1-255 characters")
		return
	}
	req.AutofillName = strings.TrimSpace(req.AutofillName)
	if len(req.AutofillName) > maxOVPNNameBytes {
		jsonError(w, http.StatusBadRequest, "autofill name must be at most 255 characters")
		return
	}

	if strings.TrimSpace(req.Config) != "" {
		raw, ok := decodeOVPNConfig(req.Config)
		if !ok {
			jsonError(w, http.StatusBadRequest, "config must be base64 of a .ovpn file (max 512 KiB)")
			return
		}
		meta := ovpn.Parse(raw)
		devType := meta.DevType
		if req.DevType != "" {
			devType = normalizeDevType(req.DevType)
		}
		authUserPass := meta.AuthUserPass
		blob, err := s.encryptProfile(id, raw)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "profile encrypt failed")
			return
		}
		_, err = s.db.Exec(`
			UPDATE openvpn_profiles
			SET name=?, autofill_name=?, description=?, requires_totp=?, allow_custom_ip=?, dev_type=?, auth_user_pass=?, config_encrypted=?
			WHERE id=?`,
			req.Name, ovpnNull(req.AutofillName), ovpnNull(req.Description), req.RequiresTOTP, req.AllowCustomIP, devType, authUserPass, blob, id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
		return
	}

	// Metadata-only update (keeps stored config + its detected auth flag).
	res, err := s.db.Exec(`
		UPDATE openvpn_profiles
		SET name=?, autofill_name=?, description=?, requires_totp=?, allow_custom_ip=?, dev_type=COALESCE(NULLIF(?,''), dev_type)
		WHERE id=?`,
		req.Name, ovpnNull(req.AutofillName), ovpnNull(req.Description), req.RequiresTOTP, req.AllowCustomIP, strings.ToLower(strings.TrimSpace(req.DevType)), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "profile not found")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/openvpn/profiles/{id}
func (s *Server) handleAdminDeleteOpenVPNProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.db.Exec("DELETE FROM openvpn_profiles WHERE id=?", id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "profile not found")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// GET /api/v1/admin/openvpn/profiles/{id}/assignments
func (s *Server) handleAdminListOpenVPNAssignments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	type row struct {
		UserID     string  `db:"user_id"     json:"user_id"`
		Username   string  `db:"username"    json:"username"`
		Email      string  `db:"email"       json:"email"`
		CustomIP   *string `db:"custom_ip"   json:"custom_ip,omitempty"`
		AssignedAt string  `db:"assigned_at" json:"assigned_at"`
	}
	rows := []row{}
	err := s.db.Select(&rows, `
		SELECT a.user_id, u.username, u.email, a.custom_ip, a.assigned_at
		FROM openvpn_profile_assignments a
		JOIN users u ON u.id = a.user_id
		WHERE a.profile_id = ?
		ORDER BY u.username`, id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, rows)
}

// POST /api/v1/admin/openvpn/profiles/{id}/assignments  {user_id, custom_ip?}
func (s *Server) handleAdminAssignOpenVPN(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		UserID   string `json:"user_id"`
		CustomIP string `json:"custom_ip"`
	}
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.UserID) == "" {
		jsonError(w, http.StatusBadRequest, "user_id required")
		return
	}
	_, err := s.db.Exec(`
		INSERT INTO openvpn_profile_assignments (id, profile_id, user_id, custom_ip)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE custom_ip=VALUES(custom_ip)`,
		newUUID(), id, req.UserID, ovpnNull(strings.TrimSpace(req.CustomIP)))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/openvpn/profiles/{id}/assignments/{userId}
func (s *Server) handleAdminUnassignOpenVPN(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	res, err := s.db.Exec(
		"DELETE FROM openvpn_profile_assignments WHERE profile_id=? AND user_id=?", id, userID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "assignment not found")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
