// Package auth implements JWT issuing and verification for the control panel.
//
// A session token is minted on login and attached to subsequent API calls via
// the Authorization: Bearer header. The same package guards the WebSocket
// gateway so browsers cannot reach workspace pods without a valid session.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Service issues and verifies signed JWTs.
type Service struct {
	secret []byte
	ttl    time.Duration
}

// NewService returns a JWT service that signs with secret and expires tokens
// after ttl. The secret should come from an env var, not be committed.
func NewService(secret []byte, ttl time.Duration) *Service {
	return &Service{secret: secret, ttl: ttl}
}

// Claims is the token payload: who the user is and when it expires.
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Issue creates a signed token for username valid for the service TTL.
func (s *Service) Issue(username string) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// Verify checks the signature, expiry, and returns the parsed claims.
func (s *Service) Verify(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
