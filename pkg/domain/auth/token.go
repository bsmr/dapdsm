package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
)

// SessionCookie is the cookie name set after successful login.
const SessionCookie = "dunemgr-session"

// TokenAuthenticator is the v1 implementation: a single shared
// secret compared by constant time.
type TokenAuthenticator struct {
	token string
}

// NewTokenAuthenticator wraps the given bearer token. The token
// is the same value the operator pastes at /login — typically
// read from <config>/token by the bootstrap.
func NewTokenAuthenticator(token string) *TokenAuthenticator {
	return &TokenAuthenticator{token: token}
}

// Authenticate checks the session cookie.
func (a *TokenAuthenticator) Authenticate(r *http.Request) (Operator, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return Operator{}, errors.New("no session cookie")
	}
	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(a.token)) != 1 {
		return Operator{}, errors.New("invalid session")
	}
	return Operator{ID: "local", Name: "operator"}, nil
}

// LoginHandler:
//   - GET /login renders the form (template lives in ui package; this
//     handler emits a minimal fallback so the package is self-
//     contained for tests).
//   - POST /login sets the cookie on a valid token.
func (a *TokenAuthenticator) LoginHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `<form method="post"><input name="token"><button>Login</button></form>`)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		got := r.PostFormValue("token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(a.token)) != 1 {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookie,
			Value:    a.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
}

// LogoutHandler clears the session cookie.
func (a *TokenAuthenticator) LogoutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     SessionCookie,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}
