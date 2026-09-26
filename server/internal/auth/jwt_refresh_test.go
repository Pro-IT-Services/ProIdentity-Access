package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-please-ignore-1234567890"

func TestIssueTokenCarriesAuthAt(t *testing.T) {
	tok, err := IssueToken("u1", "alice", false, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.AuthAt == 0 {
		t.Fatal("expected AuthAt to be set on a fresh token")
	}
	if claims.UserID != "u1" || claims.Username != "alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

// mkClaims builds claims as if the token was issued issuedAgo in the past and
// the original authentication happened authAgo in the past.
func mkClaims(issuedAgo, authAgo time.Duration) *Claims {
	now := time.Now()
	return &Claims{
		UserID:   "u1",
		Username: "alice",
		AuthAt:   now.Add(-authAgo).Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(now.Add(-issuedAgo)),
		},
	}
}

func TestRefreshIfNeeded_NotPastHalfway(t *testing.T) {
	c := mkClaims(10*time.Minute, 10*time.Minute) // 10m into a 1h token
	tok, refreshed, err := RefreshIfNeeded(c, testSecret, time.Hour, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed || tok != "" {
		t.Fatal("token under half-life should not refresh")
	}
}

func TestRefreshIfNeeded_PastHalfwayRenews(t *testing.T) {
	c := mkClaims(40*time.Minute, 40*time.Minute) // 40m into a 1h token
	tok, refreshed, err := RefreshIfNeeded(c, testSecret, time.Hour, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed || tok == "" {
		t.Fatal("token past half-life should refresh")
	}
	// The renewed token must preserve the original authentication time.
	nc, err := ParseToken(tok, testSecret)
	if err != nil {
		t.Fatalf("parse renewed: %v", err)
	}
	if nc.AuthAt != c.AuthAt {
		t.Fatalf("AuthAt should be preserved: got %d want %d", nc.AuthAt, c.AuthAt)
	}
}

func TestRefreshIfNeeded_MaxAgeCap(t *testing.T) {
	// Past half-life, but the session is older than maxAge: must not refresh so
	// the token is allowed to expire and force a full re-authentication.
	c := mkClaims(40*time.Minute, 100*24*time.Hour)
	tok, refreshed, err := RefreshIfNeeded(c, testSecret, time.Hour, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed || tok != "" {
		t.Fatal("session past max age must not refresh")
	}
}
