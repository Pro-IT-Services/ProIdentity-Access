package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"

	"proidentity/internal/devcrypto"
)

const (
	maxOVPNNameBytes   = 255
	maxOVPNConfigBytes = 512 << 10 // 512 KiB — .ovpn with inline certs is small
	profileKeySetting  = "openvpn_profile_key"
)

// openvpnProfileKey returns the 32-byte key used to encrypt profile configs at
// rest. It prefers PROIDENTITY_PROFILE_KEY (base64 of 32 bytes); otherwise it
// uses a key persisted in the settings table, generating and storing one on
// first use so dev/self-host deployments work without extra configuration.
func (s *Server) openvpnProfileKey() ([]byte, error) {
	if v := strings.TrimSpace(os.Getenv("PROIDENTITY_PROFILE_KEY")); v != "" {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("PROIDENTITY_PROFILE_KEY must be base64 of 32 bytes")
		}
		return b, nil
	}

	var stored string
	if err := s.db.Get(&stored, "SELECT `value` FROM settings WHERE `key`=?", profileKeySetting); err == nil && stored != "" {
		if b, e := base64.StdEncoding.DecodeString(stored); e == nil && len(b) == 32 {
			return b, nil
		}
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	enc := base64.StdEncoding.EncodeToString(key)
	if _, err := s.db.Exec(
		"INSERT INTO settings (`key`,`value`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `value`=VALUES(`value`)",
		profileKeySetting, enc,
	); err != nil {
		return nil, err
	}
	return key, nil
}

// encryptProfile encrypts a raw .ovpn config, binding it to the profile id (AAD).
func (s *Server) encryptProfile(profileID, plaintext string) ([]byte, error) {
	key, err := s.openvpnProfileKey()
	if err != nil {
		return nil, err
	}
	ct, err := devcrypto.Encrypt(key, []byte(plaintext), []byte(profileID))
	if err != nil {
		return nil, err
	}
	return []byte(ct), nil
}

// decryptProfile reverses encryptProfile.
func (s *Server) decryptProfile(profileID string, blob []byte) (string, error) {
	key, err := s.openvpnProfileKey()
	if err != nil {
		return "", err
	}
	plain, err := devcrypto.Decrypt(key, string(blob), []byte(profileID))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// openvpnProfileMeta is the metadata clients see for an assigned profile.
type openvpnProfileMeta struct {
	ID            string  `db:"id"              json:"id"`
	Name          string  `db:"name"            json:"name"`
	AutofillName  *string `db:"autofill_name"   json:"autofill_name,omitempty"`
	Description   *string `db:"description"     json:"description,omitempty"`
	RequiresTOTP  bool    `db:"requires_totp"   json:"requires_totp"`
	AuthUserPass  bool    `db:"auth_user_pass"  json:"auth_user_pass"`
	DevType       string  `db:"dev_type"        json:"dev_type"`
	AllowCustomIP bool    `db:"allow_custom_ip" json:"allow_custom_ip"`
	CustomIP      *string `db:"custom_ip"       json:"custom_ip,omitempty"`
}

// GET /api/v1/openvpn/profiles — profiles assigned to the current user.
func (s *Server) handleListOpenVPNProfiles(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	rows := []openvpnProfileMeta{}
	err := s.db.Select(&rows, `
		SELECT p.id, p.name, p.autofill_name, p.description, p.requires_totp, p.auth_user_pass,
		       p.dev_type, p.allow_custom_ip, a.custom_ip
		FROM openvpn_profiles p
		JOIN openvpn_profile_assignments a ON a.profile_id = p.id
		WHERE a.user_id = ?
		ORDER BY p.name`, claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, rows)
}

// GET /api/v1/openvpn/profiles/{id}/config — decrypted .ovpn for an assigned
// profile. Ownership is enforced by the assignment join.
func (s *Server) handleDownloadOpenVPNProfile(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")

	var row struct {
		openvpnProfileMeta
		ConfigEncrypted []byte `db:"config_encrypted"`
	}
	err := s.db.QueryRowx(`
		SELECT p.id, p.name, p.autofill_name, p.description, p.requires_totp, p.auth_user_pass,
		       p.dev_type, p.allow_custom_ip, a.custom_ip, p.config_encrypted
		FROM openvpn_profiles p
		JOIN openvpn_profile_assignments a ON a.profile_id = p.id
		WHERE p.id = ? AND a.user_id = ?`, id, claims.UserID).StructScan(&row)
	if err != nil {
		jsonError(w, http.StatusNotFound, "profile not found")
		return
	}

	config, err := s.decryptProfile(row.ID, row.ConfigEncrypted)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "profile decrypt failed")
		return
	}

	jsonOK(w, map[string]any{
		"id":              row.ID,
		"name":            row.Name,
		"config":          config,
		"requires_totp":   row.RequiresTOTP,
		"auth_user_pass":  row.AuthUserPass,
		"dev_type":        row.DevType,
		"allow_custom_ip": row.AllowCustomIP,
		"custom_ip":       row.CustomIP,
	})
}
