package admin

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"proidentity/internal/model"
	"proidentity/internal/pushauth"
	"proidentity/internal/requestip"
)

type PushAuthSyncResult struct {
	Checked int      `json:"checked"`
	Synced  int      `json:"synced"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

func pushAuthEnabledValue(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

func pushAuthSettings(db *sqlx.DB) (bool, string) {
	rows := []struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}{}
	_ = db.Select(&rows, "SELECT `key`, `value` FROM settings WHERE `key` IN ('push_auth_enabled', 'push_auth_api_key')")
	values := map[string]string{}
	for _, row := range rows {
		values[row.Key] = row.Value
	}
	return pushAuthEnabledValue(values["push_auth_enabled"]), strings.TrimSpace(values["push_auth_api_key"])
}

func displayNameForUser(u model.User) string {
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if name != "" {
		return name
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

func requestIP(r *http.Request) string {
	return requestip.ClientIP(r)
}

func markPushAuthSync(db *sqlx.DB, userID, status string, errText *string) {
	if errText == nil {
		_, _ = db.Exec(`
			UPDATE users
			SET push_auth_synced_at=?, push_auth_sync_status=?, push_auth_sync_error=NULL
			WHERE id=?`,
			time.Now().UTC(), status, userID)
		return
	}
	_, _ = db.Exec(`
		UPDATE users
		SET push_auth_sync_status=?, push_auth_sync_error=?
		WHERE id=?`,
		status, *errText, userID)
}

func ensurePushAuthUser(db *sqlx.DB, pc *pushauth.Client, u model.User, ip string) error {
	if strings.TrimSpace(u.Email) == "" {
		return fmt.Errorf("user %s has no email", u.Username)
	}
	status, err := pc.EnsureUser(u.Email, displayNameForUser(u), ip)
	if err != nil {
		msg := err.Error()
		markPushAuthSync(db, u.ID, "error", &msg)
		return err
	}
	markPushAuthSync(db, u.ID, status, nil)
	return nil
}

func syncPushAuthUsers(db *sqlx.DB, apiKey, ip string) PushAuthSyncResult {
	result := PushAuthSyncResult{}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		result.Errors = append(result.Errors, "push auth api key is empty")
		result.Failed = 1
		return result
	}

	users := []model.User{}
	if err := db.Select(&users, "SELECT * FROM users WHERE is_active=1 ORDER BY username"); err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Failed = 1
		return result
	}

	pc := pushauth.NewClient(apiKey)
	for _, u := range users {
		result.Checked++
		if err := ensurePushAuthUser(db, pc, u, ip); err != nil {
			result.Failed++
			line := fmt.Sprintf("%s <%s>: %v", u.Username, u.Email, err)
			result.Errors = append(result.Errors, line)
			log.Printf("push auth user sync failed: %s", line)
			continue
		}
		result.Synced++
	}
	return result
}

func maybeEnsurePushAuthUser(db *sqlx.DB, u model.User, r *http.Request) {
	enabled, apiKey := pushAuthSettings(db)
	if !enabled || apiKey == "" || strings.TrimSpace(u.Email) == "" {
		return
	}
	if u.PushAuthSyncedAt != nil && u.PushAuthSyncStatus != nil && *u.PushAuthSyncStatus != "error" {
		return
	}
	if err := ensurePushAuthUser(db, pushauth.NewClient(apiKey), u, requestIP(r)); err != nil {
		log.Printf("push auth user provisioning failed for %s <%s>: %v", u.Username, u.Email, err)
	}
}
