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
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT for the given user.
func IssueToken(userID, username string, isAdmin bool, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "proidentity",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
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
