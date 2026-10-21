package pushauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAuthRequestReportsNeedsSetup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "user_created_needs_setup",
			"message": "setup required",
		})
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	_, err := c.CreateAuthRequest("user@example.com", "Example User", "Title", "Detail", "127.0.0.1", 120)
	if err == nil || !NeedsSetup(err) {
		t.Fatalf("want a needs-setup error, got %v", err)
	}
}

// The display name goes with every real request so ProIdentity Cloud can
// create the account on first use; there is no separate provisioning call.
func TestCreateAuthRequestSendsDisplayName(t *testing.T) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sp/auth-requests" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Fatalf("missing api key header")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(AuthRequest{RequestID: "request-1", Status: "pending", ExpiresAt: 1770000000})
	}))
	defer ts.Close()

	c := NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	ar, err := c.CreateAuthRequest("user@example.com", "Example User", "ProIdentity Access", "VPN connection", "127.0.0.1", 120)
	if err != nil || ar.RequestID != "request-1" {
		t.Fatalf("CreateAuthRequest: %v %+v", err, ar)
	}
	if got["display_name"] != "Example User" || got["user_email"] != "user@example.com" {
		t.Fatalf("request body = %v", got)
	}
}
