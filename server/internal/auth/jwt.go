package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CtxKey is the context key type used to store claims. Exported so sub-packages
// can read claims from the request context without import cycles.
type CtxKey string

const ClaimsCtxKey CtxKey = "claims"

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"sub"`
	IsAdmin  bool   `json:"adm"`
	// AuthAt is the unix time of the original password/2FA authentication. It is
	// carried forward unchanged across token refreshes so the total session age
	// can be capped independently of each token's expiry.
	AuthAt int64 `json:"auth_at,omitempty"`
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT for a fresh authentication (auth time = now).
func IssueToken(userID, username string, isAdmin bool, secret string, ttl time.Duration) (string, error) {
	return IssueTokenAt(userID, username, isAdmin, secret, ttl, time.Now())
}

// IssueTokenAt creates a signed JWT while preserving an explicit original
// authentication time. Used by RefreshIfNeeded to renew a token without
// resetting the session-age anchor.
func IssueTokenAt(userID, username string, isAdmin bool, secret string, ttl time.Duration, authAt time.Time) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		AuthAt:   authAt.Unix(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "proidentity",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// RefreshIfNeeded returns a renewed token when the current one is more than
// halfway through its life, so an actively-polling client never hits the expiry
// wall. It preserves the original authentication time and refuses to refresh
// once the session exceeds maxAge, letting the token expire and forcing a full
// re-authentication. Returns ("", false, nil) when no refresh is warranted.
func RefreshIfNeeded(claims *Claims, secret string, ttl, maxAge time.Duration) (string, bool, error) {
	now := time.Now()

	// Anchor the session age. Legacy tokens predate AuthAt; fall back to their
	// issue time so they still benefit from (and are bounded by) refresh.
	authAt := time.Unix(claims.AuthAt, 0)
	if claims.AuthAt == 0 {
		if claims.IssuedAt != nil {
			authAt = claims.IssuedAt.Time
		} else {
			authAt = now
		}
	}
	if maxAge > 0 && now.Sub(authAt) >= maxAge {
		return "", false, nil
	}

	// Only renew past the halfway mark to avoid re-issuing on every poll.
	if claims.IssuedAt != nil && now.Sub(claims.IssuedAt.Time) < ttl/2 {
		return "", false, nil
	}

	tok, err := IssueTokenAt(claims.UserID, claims.Username, claims.IsAdmin, secret, ttl, authAt)
	if err != nil {
		return "", false, err
	}
	return tok, true, nil
}

// ParseToken validates and parses a JWT string.
func ParseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		},
		jwt.WithIssuer("proidentity"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
