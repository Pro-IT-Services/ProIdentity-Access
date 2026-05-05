package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"proidentity/internal/pushauth"
)

func resetPushAuthStateForTest() {
	pushPendingMu.Lock()
	pushPending = map[string]pendingPushAuth{}
	pushPendingMu.Unlock()
}

func TestFindReusablePendingPushAuthReturnsPendingRequest(t *testing.T) {
	resetPushAuthStateForTest()
	t.Cleanup(resetPushAuthStateForTest)

	var polls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&polls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"pending"}`))
	}))
	defer ts.Close()

	pc := pushauth.NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	pending := rememberPushAuth("push-1", "user-1", "session", "Connect to IPC CORP TEST", 120)

	requestID, got, ok := findReusablePendingPushAuth(pc, "user-1", "session", "Connect to IPC CORP TEST")
	if !ok {
		t.Fatal("expected reusable pending push request")
	}
	if requestID != "push-1" {
		t.Fatalf("request id = %q, want push-1", requestID)
	}
	if got.ExpiresAt != pending.ExpiresAt {
		t.Fatalf("expires = %v, want %v", got.ExpiresAt, pending.ExpiresAt)
	}
	if atomic.LoadInt32(&polls) != 1 {
		t.Fatalf("poll count = %d, want 1", polls)
	}
}

func TestFindReusablePendingPushAuthDropsDeniedRequest(t *testing.T) {
	resetPushAuthStateForTest()
	t.Cleanup(resetPushAuthStateForTest)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"denied"}`))
	}))
	defer ts.Close()

	pc := pushauth.NewClientWithBaseURL("test-key", ts.URL, ts.Client())
	rememberPushAuth("push-1", "user-1", "session", "Connect to IPC CORP TEST", 120)

	if requestID, _, ok := findReusablePendingPushAuth(pc, "user-1", "session", "Connect to IPC CORP TEST"); ok {
		t.Fatalf("reused denied request %q", requestID)
	}
	if _, ok := lookupPendingPushAuth("push-1"); ok {
		t.Fatal("denied request stayed in pending registry")
	}
}
