// Package auth gates access to dunemgr's HTTP surface. v1 ships a
// Token authenticator; v2 adds OIDC behind the same interface.
package auth

import "net/http"

// Operator identifies the human who issued the current request.
// Stored in request context after successful authentication, and
// written to every audit entry.
type Operator struct {
	ID    string   // v1: "local"; v2: OIDC `sub`
	Name  string   // v1: "operator"; v2: OIDC display name
	Roles []string // v2-only; v1 leaves nil
}

// Authenticator is the gate.
type Authenticator interface {
	// Authenticate inspects the request and returns the operator,
	// or an error if unauthenticated.
	Authenticate(r *http.Request) (Operator, error)

	// LoginHandler serves the login page (v1) or initiates the
	// OIDC redirect (v2).
	LoginHandler() http.Handler

	// LogoutHandler clears the session and redirects to /login.
	LogoutHandler() http.Handler
}
