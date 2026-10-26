// CSRF guard: cookie-authenticated mutations must prove
// JavaScript origin via the X-CSRF-Token header.
//
// Model: API routes authenticate with the Authorization: Bearer header,
// which browsers never attach cross-site, so Bearer requests skip the
// guard. The session cookie (SameSite=Lax) is read only by the gateway's
// GET navigation today — but any cookie-carrying POST/PUT/PATCH/DELETE
// (current or future, e.g. the R4 SPA) must present the per-login token
// issued alongside the JWT, or the request is rejected with 403 before
// reaching a handler. The login endpoint is exempt: it issues tokens, so
// it can never require one — otherwise a stale session cookie would lock
// users out of re-login. Other pre-auth endpoints (register, polls,
// diagnostics) carry no cookie in practice and stay guarded.
package httpapi

import (
	"net/http"

	"github.com/ldndrc/control-panel/internal/auth"
)

// CSRFHeader is the request header carrying the per-login CSRF token.
const CSRFHeader = "X-CSRF-Token"

// guardCSRF rejects cookie-authenticated mutations without a valid token.
func guardCSRF(authSvc *auth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The login endpoint issues tokens, so it can never require
		// one: a stale session cookie must not lock users out of
		// re-login (seen live: 403 "csrf token missing or invalid"
		// on re-login with an old cookie, handler never reached).
		if r.URL.Path == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(authSvc.CookieName())
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r) // Bearer path: no ambient credential
			return
		}
		if !authSvc.VerifyCSRF(cookie.Value, r.Header.Get(CSRFHeader)) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf token missing or invalid"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
