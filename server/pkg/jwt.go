package pkg

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenSigner signs and verifies JWT tokens using HMAC-SHA256.
type TokenSigner struct {
	key    string
	method *jwt.SigningMethodHMAC
}

// NewTokenSigner returns a TokenSigner backed by the provided secret key.
func NewTokenSigner(key string) *TokenSigner {
	return &TokenSigner{
		key:    key,
		method: jwt.SigningMethodHS256,
	}
}

// Sign creates a signed JWT with the given claims map.
func (ts *TokenSigner) Sign(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(ts.method, claims)
	return token.SignedString([]byte(ts.key))
}

// Verify parses and validates a JWT string, returning its claims on success.
func (ts *TokenSigner) Verify(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(ts.key), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid or expired token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// StandardClaims builds a MapClaims with uid, exp, and iat pre-populated.
func StandardClaims(uid string, ttl time.Duration) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"uid": uid,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
}
