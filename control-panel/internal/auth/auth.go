// Package auth implements JWT issuing and verification for the control panel.
//
// A session token is minted on login and attached to subsequent API calls via
// the Authorization: Bearer header. The same package guards the WebSocket
// gateway so browsers cannot reach workspace pods without a valid session.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
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

// CookieName returns the browser session cookie name.
func (s *Service) CookieName() string {
	if name := os.Getenv("SESSION_COOKIE_NAME"); name != "" {
		return name
	}
	return "ldndrc_session"
}

// SetSessionCookie writes the JWT as an HttpOnly cookie for browser tools.
func (s *Service) SetSessionCookie(w http.ResponseWriter, token string) {
	maxAge := int(s.ttl.Seconds())
	cookie := &http.Cookie{
		Name:     s.CookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("COOKIE_SECURE") == "true",
	}
	if domain := os.Getenv("COOKIE_DOMAIN"); domain != "" {
		cookie.Domain = domain
	}
	http.SetCookie(w, cookie)
}

// ClearSessionCookie removes the browser session cookie.
func (s *Service) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.CookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("COOKIE_SECURE") == "true",
	})
}

// IssueCSRF derives a per-login CSRF token bound to tokenString. It is
// HMAC(JWT_SECRET, "csrf:"+jwt): stateless (no store), unforgeable
// without the secret, and rotates whenever the session is re-issued.
// The browser keeps it in JS memory (never a cookie) and sends it back
// as the X-CSRF-Token header on cookie-authenticated mutations.
func (s *Service) IssueCSRF(tokenString string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte("csrf:" + tokenString))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyCSRF reports whether csrf is the token bound to tokenString.
func (s *Service) VerifyCSRF(tokenString, csrf string) bool {
	want := s.IssueCSRF(tokenString)
	got, err := hex.DecodeString(csrf)
	wantBytes, _ := hex.DecodeString(want)
	if err != nil || len(got) != len(wantBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(got, wantBytes) == 1
}

// VerifyCookie authenticates the configured session cookie from a request.
func (s *Service) VerifyCookie(r *http.Request) (*Claims, error) {
	cookie, err := r.Cookie(s.CookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, errors.New("session cookie missing")
	}
	return s.Verify(cookie.Value)
}
