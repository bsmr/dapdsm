package fls

import "testing"

func TestOperatorZeroValue(t *testing.T) {
	var op Operator
	if op.ID != "" || op.Name != "" {
		t.Errorf("Operator zero value not empty: %+v", op)
	}
}

func TestAuthenticatorInterface(t *testing.T) {
	// Compile-time: TokenAuthenticator implements Authenticator
	// (this test forces the package to compile; the concrete type
	// is wired in Task E2).
	var _ = Authenticator(nil)
}
