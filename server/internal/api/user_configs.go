package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	maxConfigNameBytes   = 128
	maxStoredConfigBytes = 256 << 10
)

// GET /api/v1/user/config-key
// Returns the user's 32-byte config encryption key (base64).
// Auto-generates and persists the key if it doesn't exist yet.
func (s *Server) handleGetConfigKey(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)

	var keyBytes []byte
	err := s.db.Get(&keyBytes, "SELECT config_key FROM users WHERE id=?", claims.UserID)
	if err != nil || len(keyBytes) != 32 {
		// Generate a new key
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to generate key")
			return
		}
		if _, err := s.db.Exec("UPDATE users SET config_key=? WHERE id=?", key, claims.UserID); err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to store key")
			return
		}
		keyBytes = key
	}

	jsonOK(w, map[string]string{
		"key": base64.StdEncoding.EncodeToString(keyBytes),
	})
}

// GET /api/v1/user/configs
// Lists all config records for the authenticated user (metadata only, no content).
func (s *Server) handleListUserConfigs(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)

	rows, err := s.db.Queryx(
		"SELECT id, name, created_at FROM user_configs WHERE user_id=? ORDER BY created_at DESC",
		claims.UserID,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type configMeta struct {
		ID        string `db:"id"        json:"id"`
		Name      string `db:"name"      json:"name"`
		CreatedAt string `db:"created_at" json:"created_at"`
	}
	var configs []configMeta
	for rows.Next() {
		var c configMeta
		if err := rows.StructScan(&c); err != nil {
			continue
		}
		configs = append(configs, c)
	}
	if configs == nil {
		configs = []configMeta{}
	}
	jsonOK(w, configs)
}

// POST /api/v1/user/configs
// Uploads an encrypted config. Body: {name, encrypted_content (base64)}.
func (s *Server) handleUploadUserConfig(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)

	var req struct {
		Name             string `json:"name"`
		EncryptedContent string `json:"encrypted_content"` // base64
	}
	if err := decode(r, &req); err != nil || req.Name == "" || req.EncryptedContent == "" {
		jsonError(w, http.StatusBadRequest, "name and encrypted_content required")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > maxConfigNameBytes {
		jsonError(w, http.StatusBadRequest, "name must be 1-128 characters")
		return
	}

	blob, err := base64.StdEncoding.DecodeString(req.EncryptedContent)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "encrypted_content must be base64")
		return
	}
	if len(blob) == 0 || len(blob) > maxStoredConfigBytes {
		jsonError(w, http.StatusBadRequest, "encrypted_content is too large")
		return
	}

	id := newUUID()
	_, err = s.db.Exec(
		"INSERT INTO user_configs (id, user_id, name, encrypted_content) VALUES (?, ?, ?, ?)",
		id, claims.UserID, req.Name, blob,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]string{"id": id})
}

// GET /api/v1/user/configs/{id}
// Downloads the encrypted content for a config.
func (s *Server) handleDownloadUserConfig(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")

	var row struct {
		Name             string `db:"name"`
		EncryptedContent []byte `db:"encrypted_content"`
	}
	err := s.db.QueryRowx(
		"SELECT name, encrypted_content FROM user_configs WHERE id=? AND user_id=?",
		id, claims.UserID,
	).StructScan(&row)
	if err != nil {
		jsonError(w, http.StatusNotFound, "config not found")
		return
	}

	jsonOK(w, map[string]string{
		"id":                id,
		"name":              row.Name,
		"encrypted_content": base64.StdEncoding.EncodeToString(row.EncryptedContent),
	})
}

// GET /api/v1/admin/user-configs
// Returns all stored configs across all users (metadata only, no content).
func (s *Server) handleAdminListUserConfigs(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID        string `db:"id"         json:"id"`
		Name      string `db:"name"       json:"name"`
		CreatedAt string `db:"created_at" json:"created_at"`
		UserID    string `db:"user_id"    json:"user_id"`
		Username  string `db:"username"   json:"username"`
		Email     string `db:"email"      json:"email"`
	}
	rows, err := s.db.Queryx(`
		SELECT uc.id, uc.name, uc.created_at, u.id AS user_id, u.username, u.email
		FROM user_configs uc
		JOIN users u ON u.id = uc.user_id
		ORDER BY u.username, uc.created_at DESC`,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var configs []row
	for rows.Next() {
		var r row
		if err := rows.StructScan(&r); err != nil {
			continue
		}
		configs = append(configs, r)
	}
	if configs == nil {
		configs = []row{}
	}
	jsonOK(w, configs)
}

// DELETE /api/v1/admin/user-configs/{id}
// Admin can delete any user's stored config.
func (s *Server) handleAdminDeleteUserConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.db.Exec("DELETE FROM user_configs WHERE id=?", id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "config not found")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// DELETE /api/v1/user/configs/{id}
func (s *Server) handleDeleteUserConfig(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	id := chi.URLParam(r, "id")

	res, err := s.db.Exec(
		"DELETE FROM user_configs WHERE id=? AND user_id=?",
		id, claims.UserID,
	)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		jsonError(w, http.StatusNotFound, "config not found")
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}
