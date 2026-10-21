package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"proidentity/internal/wgconf"
)

// WireGuard profiles: admin-uploaded .conf files for WireGuard servers this
// product doesn't run, assigned to users like OpenVPN profiles.
//
// A profile is a single WireGuard peer, so only one device may have it up.
// Devices claim it before connecting and renew the claim while connected
// (heartbeat); a claim not renewed within wgClaimTTLSeconds lapses, which
// covers devices that crashed or went offline. The config is only handed out
// with a successful claim.

const (
	wgClaimTTLSeconds  = 180
	maxWGConfigBytes   = 64 << 10
	maxWGProfileName   = 255
	errWGNeedsDevice   = "a registered device is required"
	errWGProfileAbsent = "profile not found"
)

// wgHolder is who has a profile up right now.
type wgHolder struct {
	Name        string `json:"name"` // "First Last", else the username
	Username    string `json:"username"`
	Device      string `json:"device"`       // device name of the installation
	HeldSeconds int64  `json:"held_seconds"` // how long it has been up
	You         bool   `json:"you"`          // the requesting user
	ThisDevice  bool   `json:"this_device"`  // the requesting device
}

type wgProfileRow struct {
	ID          string         `db:"id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	Shared      bool           `db:"shared"`
	Address     sql.NullString `db:"address"`
	Endpoint    sql.NullString `db:"endpoint"`
	Held        bool           `db:"held"`
	HolderUser  sql.NullString `db:"holder_user_id"`
	HolderInst  sql.NullString `db:"holder_installation_id"`
	HolderDev   sql.NullString `db:"holder_device_name"`
	HeldSeconds sql.NullInt64  `db:"held_seconds"`
	HUsername   sql.NullString `db:"holder_username"`
	HFirst      sql.NullString `db:"holder_first_name"`
	HLast       sql.NullString `db:"holder_last_name"`
}

// wgProfileSelect is shared by the user and admin listings; `held` is true
// only while the claim is fresh.
var wgProfileSelect = fmt.Sprintf(`
	SELECT p.id, p.name, p.description, p.shared, p.address, p.endpoint,
	       (p.holder_installation_id IS NOT NULL AND p.holder_seen >= NOW() - INTERVAL %d SECOND) AS held,
	       p.holder_user_id, p.holder_installation_id, p.holder_device_name,
	       TIMESTAMPDIFF(SECOND, p.held_since, NOW()) AS held_seconds,
	       u.username AS holder_username, u.first_name AS holder_first_name, u.last_name AS holder_last_name
	FROM wireguard_profiles p
	LEFT JOIN users u ON u.id = p.holder_user_id`, wgClaimTTLSeconds)

func (row wgProfileRow) holder(userID, deviceID string) *wgHolder {
	if !row.Held {
		return nil
	}
	name := strings.TrimSpace(row.HFirst.String + " " + row.HLast.String)
	if name == "" {
		name = row.HUsername.String
	}
	return &wgHolder{
		Name:        name,
		Username:    row.HUsername.String,
		Device:      row.HolderDev.String,
		HeldSeconds: row.HeldSeconds.Int64,
		You:         userID != "" && row.HolderUser.String == userID,
		ThisDevice:  deviceID != "" && row.HolderInst.String == deviceID,
	}
}

// wgProfileView is what clients see for an assigned profile. No keys.
type wgProfileView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Shared      bool      `json:"shared"`
	Address     string    `json:"address,omitempty"`
	Endpoint    string    `json:"endpoint,omitempty"`
	InUseBy     *wgHolder `json:"in_use_by,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// GET /api/v1/wireguard/profiles: profiles assigned to the current user.
func (s *Server) handleListWGProfiles(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	deviceID := r.Header.Get("X-Device-ID")
	rows := []wgProfileRow{}
	err := s.db.Select(&rows, wgProfileSelect+`
		JOIN wireguard_profile_assignments a ON a.profile_id = p.id AND a.user_id = ?
		ORDER BY p.name`, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]wgProfileView, 0, len(rows))
	for _, row := range rows {
		out = append(out, wgProfileView{
			ID: row.ID, Name: row.Name, Description: row.Description.String, Shared: row.Shared,
			Address: row.Address.String, Endpoint: row.Endpoint.String,
			InUseBy: row.holder(claims.UserID, deviceID),
		})
	}
	jsonOK(w, out)
}

// claimWGProfile takes or renews the lock for this device. It reports the
// current holder when someone else has the profile up.
func (s *Server) claimWGProfile(r *http.Request, profileID string) (holder *wgHolder, status int, msg string) {
	claims := claimsFrom(r)
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		return nil, http.StatusBadRequest, errWGNeedsDevice
	}
	var assigned int
	if err := s.db.Get(&assigned, `SELECT COUNT(*) FROM wireguard_profile_assignments WHERE profile_id=? AND user_id=?`,
		profileID, claims.UserID); err != nil {
		return nil, http.StatusInternalServerError, err.Error()
	}
	if assigned == 0 {
		return nil, http.StatusNotFound, errWGProfileAbsent
	}
	var deviceName string
	if err := s.db.Get(&deviceName, `SELECT device_name FROM installations WHERE id=? AND is_active=1`, deviceID); err != nil {
		return nil, http.StatusBadRequest, errWGNeedsDevice
	}

	// held_since is set first: MySQL applies SET assignments left to right,
	// so the comparison still sees the previous holder.
	res, err := s.db.Exec(`
		UPDATE wireguard_profiles
		SET held_since = IF(holder_installation_id <=> ?, held_since, NOW()),
		    holder_user_id = ?, holder_installation_id = ?, holder_device_name = ?, holder_seen = NOW()
		WHERE id = ?
		  AND (holder_installation_id IS NULL OR holder_installation_id = ?
		       OR holder_seen IS NULL OR holder_seen < NOW() - INTERVAL ? SECOND)`,
		deviceID, claims.UserID, deviceID, deviceName, profileID, deviceID, wgClaimTTLSeconds)
	if err != nil {
		return nil, http.StatusInternalServerError, err.Error()
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil, http.StatusOK, ""
	}
	// MySQL reports 0 rows when nothing changed: same holder within the same
	// second. Look before calling it a conflict.
	var row wgProfileRow
	if err := s.db.Get(&row, wgProfileSelect+` WHERE p.id = ?`, profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, http.StatusNotFound, errWGProfileAbsent
		}
		return nil, http.StatusInternalServerError, err.Error()
	}
	h := row.holder(claims.UserID, deviceID)
	if h == nil || h.ThisDevice {
		return nil, http.StatusOK, ""
	}
	return h, http.StatusConflict, "in use"
}

func (s *Server) writeWGClaimError(w http.ResponseWriter, holder *wgHolder, status int, msg string) {
	if status == http.StatusConflict && holder != nil {
		text := "In use by " + holder.Name
		if holder.Device != "" {
			text += " on " + holder.Device
		}
		writeJSON(w, http.StatusConflict, map[string]any{"error": text, "in_use_by": holder})
		return
	}
	jsonError(w, status, msg)
}

// POST /api/v1/wireguard/profiles/{id}/claim: take the profile for this
// device and get its config.
func (s *Server) handleClaimWGProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if holder, status, msg := s.claimWGProfile(r, id); status != http.StatusOK {
		s.writeWGClaimError(w, holder, status, msg)
		return
	}
	var row struct {
		Name string `db:"name"`
		Blob []byte `db:"config_encrypted"`
	}
	if err := s.db.Get(&row, `SELECT name, config_encrypted FROM wireguard_profiles WHERE id=?`, id); err != nil {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	config, err := s.decryptProfile(id, row.Blob)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "profile decrypt failed")
		return
	}
	jsonOK(w, map[string]any{"id": id, "name": row.Name, "config": config, "lease_seconds": wgClaimTTLSeconds})
}

// POST /api/v1/wireguard/profiles/{id}/heartbeat: renew the claim while
// connected. Unlike claim it never takes a free profile: a claim that lapsed
// while the device was offline is renewed if nobody took it, but one an
// admin freed stays free. 409 means the profile is no longer this device's
// (with in_use_by when another device has it); 404 that it was unassigned or
// deleted. Either way the device must disconnect.
func (s *Server) handleHeartbeatWGProfile(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		jsonError(w, http.StatusBadRequest, errWGNeedsDevice)
		return
	}
	var assigned int
	if err := s.db.Get(&assigned, `SELECT COUNT(*) FROM wireguard_profile_assignments WHERE profile_id=? AND user_id=?`,
		id, claims.UserID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if assigned == 0 {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	if _, err := s.db.Exec(`UPDATE wireguard_profiles SET holder_seen=NOW() WHERE id=? AND holder_installation_id=?`,
		id, deviceID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Rows affected is 0 when holder_seen didn't change (same second), so
	// read the holder back instead of trusting it.
	var holderInst sql.NullString
	if err := s.db.Get(&holderInst, `SELECT holder_installation_id FROM wireguard_profiles WHERE id=?`, id); err != nil {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	if holderInst.String == deviceID {
		jsonOK(w, map[string]any{"ok": true, "lease_seconds": wgClaimTTLSeconds})
		return
	}
	var row wgProfileRow
	if err := s.db.Get(&row, wgProfileSelect+` WHERE p.id = ?`, id); err == nil {
		if h := row.holder(claims.UserID, deviceID); h != nil {
			s.writeWGClaimError(w, h, http.StatusConflict, "in use")
			return
		}
	}
	jsonError(w, http.StatusConflict, "released")
}

// POST /api/v1/wireguard/profiles/{id}/release: this device disconnected.
func (s *Server) handleReleaseWGProfile(w http.ResponseWriter, r *http.Request) {
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		jsonError(w, http.StatusBadRequest, errWGNeedsDevice)
		return
	}
	_, err := s.db.Exec(`
		UPDATE wireguard_profiles
		SET holder_user_id=NULL, holder_installation_id=NULL, holder_device_name=NULL, held_since=NULL, holder_seen=NULL
		WHERE id=? AND holder_installation_id=?`, chi.URLParam(r, "id"), deviceID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// --- Admin ---

// decodeWGConfig accepts the uploaded .conf as base64 and validates it.
func decodeWGConfig(v string) (string, error) {
	raw, ok := decodeBase64Config(v, maxWGConfigBytes)
	if !ok {
		return "", errors.New("config must be base64 of a WireGuard .conf file (max 64 KiB)")
	}
	return raw, nil
}

func parseWGConfig(raw string) (wgconf.Metadata, error) {
	return wgconf.Parse(raw)
}

type wgAdminProfile struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Shared        bool      `json:"shared"`
	Address       string    `json:"address,omitempty"`
	Endpoint      string    `json:"endpoint,omitempty"`
	AssignedCount int       `json:"assigned_count"`
	InUseBy       *wgHolder `json:"in_use_by,omitempty"`
}

// GET /api/v1/admin/wireguard/profiles
func (s *Server) handleAdminListWGProfiles(w http.ResponseWriter, r *http.Request) {
	rows := []struct {
		wgProfileRow
		AssignedCount int `db:"assigned_count"`
	}{}
	err := s.db.Select(&rows, strings.Replace(wgProfileSelect, "SELECT p.id,",
		"SELECT (SELECT COUNT(*) FROM wireguard_profile_assignments a WHERE a.profile_id = p.id) AS assigned_count, p.id,", 1)+`
		ORDER BY p.name`)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]wgAdminProfile, 0, len(rows))
	for _, row := range rows {
		out = append(out, wgAdminProfile{
			ID: row.ID, Name: row.Name, Description: row.Description.String, Shared: row.Shared,
			Address: row.Address.String, Endpoint: row.Endpoint.String,
			AssignedCount: row.AssignedCount, InUseBy: row.holder("", ""),
		})
	}
	jsonOK(w, out)
}

// POST /api/v1/admin/wireguard/profiles
func (s *Server) handleAdminCreateWGProfile(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Config      string `json:"config"` // base64 of the .conf
		Shared      bool   `json:"shared"`
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxWGProfileName {
		jsonError(w, http.StatusBadRequest, "name must be 1-255 characters")
		return
	}
	raw, err := decodeWGConfig(req.Config)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	meta, err := parseWGConfig(raw)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	id := newUUID()
	blob, err := s.encryptProfile(id, raw)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "profile encrypt failed")
		return
	}
	_, err = s.db.Exec(`
		INSERT INTO wireguard_profiles (id, name, description, config_encrypted, shared, address, endpoint, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Name, ovpnNull(strings.TrimSpace(req.Description)), blob, req.Shared,
		ovpnNull(meta.Address), ovpnNull(meta.Endpoint), claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]string{"id": id})
}

// PUT /api/v1/admin/wireguard/profiles/{id}
func (s *Server) handleAdminUpdateWGProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Shared      bool   `json:"shared"`
		Config      string `json:"config"` // optional base64 replacement
	}
	if err := decode(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxWGProfileName {
		jsonError(w, http.StatusBadRequest, "name must be 1-255 characters")
		return
	}
	if !req.Shared {
		var n int
		if err := s.db.Get(&n, `SELECT COUNT(*) FROM wireguard_profile_assignments WHERE profile_id=?`, id); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n > 1 {
			jsonError(w, http.StatusConflict, "This profile is assigned to several users. Remove all but one before turning sharing off.")
			return
		}
	}

	if strings.TrimSpace(req.Config) != "" {
		raw, err := decodeWGConfig(req.Config)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		meta, err := parseWGConfig(raw)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		blob, err := s.encryptProfile(id, raw)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "profile encrypt failed")
			return
		}
		res, err := s.db.Exec(`
			UPDATE wireguard_profiles SET name=?, description=?, shared=?, config_encrypted=?, address=?, endpoint=?
			WHERE id=?`,
			req.Name, ovpnNull(strings.TrimSpace(req.Description)), req.Shared, blob,
			ovpnNull(meta.Address), ovpnNull(meta.Endpoint), id)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			jsonError(w, http.StatusNotFound, errWGProfileAbsent)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
		return
	}

	var exists int
	if err := s.db.Get(&exists, `SELECT COUNT(*) FROM wireguard_profiles WHERE id=?`, id); err != nil || exists == 0 {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	if _, err := s.db.Exec(`UPDATE wireguard_profiles SET name=?, description=?, shared=? WHERE id=?`,
		req.Name, ovpnNull(strings.TrimSpace(req.Description)), req.Shared, id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/wireguard/profiles/{id}
func (s *Server) handleAdminDeleteWGProfile(w http.ResponseWriter, r *http.Request) {
	res, err := s.db.Exec("DELETE FROM wireguard_profiles WHERE id=?", chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/wireguard/profiles/{id}/holder: free a profile whose
// holder can't release it (lost laptop). That device is told on its next
// heartbeat and disconnects.
func (s *Server) handleAdminReleaseWGProfile(w http.ResponseWriter, r *http.Request) {
	_, err := s.db.Exec(`
		UPDATE wireguard_profiles
		SET holder_user_id=NULL, holder_installation_id=NULL, holder_device_name=NULL, held_since=NULL, holder_seen=NULL
		WHERE id=?`, chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// GET /api/v1/admin/wireguard/profiles/{id}/assignments
func (s *Server) handleAdminListWGAssignments(w http.ResponseWriter, r *http.Request) {
	type row struct {
		UserID     string `db:"user_id"     json:"user_id"`
		Username   string `db:"username"    json:"username"`
		Email      string `db:"email"       json:"email"`
		AssignedAt string `db:"assigned_at" json:"assigned_at"`
	}
	rows := []row{}
	err := s.db.Select(&rows, `
		SELECT a.user_id, u.username, u.email, a.assigned_at
		FROM wireguard_profile_assignments a
		JOIN users u ON u.id = a.user_id
		WHERE a.profile_id = ?
		ORDER BY u.username`, chi.URLParam(r, "id"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, rows)
}

// POST /api/v1/admin/wireguard/profiles/{id}/assignments  {user_id}
// A profile not marked shared belongs to one user only.
func (s *Server) handleAdminAssignWG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decode(r, &req); err != nil || strings.TrimSpace(req.UserID) == "" {
		jsonError(w, http.StatusBadRequest, "user_id required")
		return
	}
	var shared bool
	if err := s.db.Get(&shared, `SELECT shared FROM wireguard_profiles WHERE id=?`, id); err != nil {
		jsonError(w, http.StatusNotFound, errWGProfileAbsent)
		return
	}
	if !shared {
		var other string
		err := s.db.Get(&other, `
			SELECT u.username FROM wireguard_profile_assignments a JOIN users u ON u.id = a.user_id
			WHERE a.profile_id=? AND a.user_id<>? LIMIT 1`, id, req.UserID)
		if err == nil {
			jsonError(w, http.StatusConflict, "This profile isn't shared and is already assigned to "+other+". Mark it as shared, or remove that user first.")
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	_, err := s.db.Exec(`
		INSERT INTO wireguard_profile_assignments (id, profile_id, user_id) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE assigned_at=assigned_at`, newUUID(), id, req.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/admin/wireguard/profiles/{id}/assignments/{userId}
// Also frees the profile if that user has it up: their device disconnects
// on its next heartbeat.
func (s *Server) handleAdminUnassignWG(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	res, err := s.db.Exec(`DELETE FROM wireguard_profile_assignments WHERE profile_id=? AND user_id=?`, id, userID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "assignment not found")
		return
	}
	_, _ = s.db.Exec(`
		UPDATE wireguard_profiles
		SET holder_user_id=NULL, holder_installation_id=NULL, holder_device_name=NULL, held_since=NULL, holder_seen=NULL
		WHERE id=? AND holder_user_id=?`, id, userID)
	jsonOK(w, map[string]bool{"ok": true})
}
