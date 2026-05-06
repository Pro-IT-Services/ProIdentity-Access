package api

import (
	"net/http/httptest"
	"testing"
)

func TestRemoteAddrDoesNotTrustLoopbackByDefault(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")
	t.Setenv("PROIDENTITY_DISABLE_X_FORWARDED_FOR", "")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	r.Header.Set("X-Real-IP", "203.0.113.11")

	if got := remoteAddr(r); got != "127.0.0.1" {
		t.Fatalf("remoteAddr = %q, want loopback peer", got)
	}
}

func TestRemoteAddrTrustsExplicitProxyRealIP(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "127.0.0.1")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")
	t.Setenv("PROIDENTITY_DISABLE_X_FORWARDED_FOR", "")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "198.51.100.200")
	r.Header.Set("X-Real-IP", "203.0.113.11")

	if got := remoteAddr(r); got != "198.51.100.200" {
		t.Fatalf("remoteAddr = %q, want X-Forwarded-For client", got)
	}
}
