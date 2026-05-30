package auth

import "net/http"

// safeMethods do not change server state — no Origin check needed.
var safeMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// OriginGuard rejects state-changing requests whose Origin header
// does not match the configured server host. Mitigates CSRF and
// DNS-rebinding on localhost binds.
//
// expectedHost is the host:port the server listens on, used as
// the canonical Origin.
func OriginGuard(expectedHost string) func(http.Handler) http.Handler {
	expected := "http://" + expectedHost
	expectedHTTPS := "https://" + expectedHost
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if safeMethods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}
			o := r.Header.Get("Origin")
			if o != expected && o != expectedHTTPS {
				http.Error(w, "origin mismatch", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth wraps a handler so only authenticated requests pass.
// Unauthenticated requests are 303-redirected to /login.
func RequireAuth(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := a.Authenticate(r); err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
