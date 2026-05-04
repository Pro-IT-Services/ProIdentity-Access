package pushauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnsureUserTreatsSetupRequiredAsProvisioned(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sp/auth-requests" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "test-key" {
			t.Fatalf("missing api key header: %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":       "user_created_needs_setup",
			"message":    "setup required",
			"user_email": "user@example.com",
		})
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	status, err := c.EnsureUser("user@example.com", "Example User", "127.0.0.1")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if status != "user_created_needs_setup" {
		t.Fatalf("status = %q", status)
	}
}

func TestCreateAuthRequestRejectsSetupRequiredForLoginFlow(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "user_created_needs_setup",
			"message": "setup required",
		})
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	if _, err := c.CreateAuthRequest("user@example.com", "Title", "Detail", "127.0.0.1", 120); err == nil {
		t.Fatal("expected setup-required response to be rejected for auth flow")
	}
}

func TestEnsureUserReturnsPendingForExistingUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(AuthRequest{
			RequestID: "request-1",
			Status:    "pending",
			ExpiresAt: 1770000000,
		})
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	status, err := c.EnsureUser("user@example.com", "Example User", "127.0.0.1")
	if err != nil {
		t.Fatalf("EnsureUser returned error: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status = %q", status)
	}
}
