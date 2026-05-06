package requestip

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresProxyHeadersFromUntrustedPeer(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.9:44321"
	r.Header.Set("X-Forwarded-For", "203.0.113.10")
	r.Header.Set("X-Real-IP", "203.0.113.11")

	if got := ClientIP(r); got != "10.0.0.9" {
		t.Fatalf("ClientIP = %q, want direct peer", got)
	}
}

func TestClientIPUsesRightmostUntrustedForwardedFor(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "10.0.0.0/8")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")
	t.Setenv("PROIDENTITY_DISABLE_X_FORWARDED_FOR", "")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.20:44321"
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.77, 10.0.0.10")

	if got := ClientIP(r); got != "203.0.113.77" {
		t.Fatalf("ClientIP = %q, want rightmost untrusted address", got)
	}
}

func TestClientIPUsesForwardedHeader(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "127.0.0.1")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:44321"
	r.Header.Set("Forwarded", `for=198.51.100.44;proto=https;host=vpn.example.com`)

	if got := ClientIP(r); got != "198.51.100.44" {
		t.Fatalf("ClientIP = %q, want Forwarded for address", got)
	}
}

func TestClientIPFallsBackToRealIPWhenForwardedForDisabled(t *testing.T) {
	t.Setenv("PROIDENTITY_TRUSTED_PROXIES", "127.0.0.1")
	t.Setenv("PROIDENTITY_TRUST_LOOPBACK_PROXY", "")
	t.Setenv("PROIDENTITY_DISABLE_X_FORWARDED_FOR", "1")

	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:44321"
	r.Header.Set("X-Forwarded-For", "198.51.100.44")
	r.Header.Set("X-Real-IP", "203.0.113.9")

	if got := ClientIP(r); got != "203.0.113.9" {
		t.Fatalf("ClientIP = %q, want X-Real-IP fallback", got)
	}
}
