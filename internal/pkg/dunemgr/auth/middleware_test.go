package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestOriginGuardAllowsSameHost(t *testing.T) {
	mw := OriginGuard("127.0.0.1:8765")
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765/x", nil)
	r.Header.Set("Origin", "http://127.0.0.1:8765")
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("same-origin POST: status %d, want 200", w.Result().StatusCode)
	}
}

func TestOriginGuardBlocksCrossOrigin(t *testing.T) {
	mw := OriginGuard("127.0.0.1:8765")
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765/x", nil)
	r.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin POST: status %d, want 403", w.Result().StatusCode)
	}
}

func TestOriginGuardAllowsGET(t *testing.T) {
	mw := OriginGuard("127.0.0.1:8765")
	r := httptest.NewRequest("GET", "http://127.0.0.1:8765/x", nil)
	r.Header.Set("Origin", "http://evil.example.com")
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("cross-origin GET: status %d, want 200 (safe method)", w.Result().StatusCode)
	}
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	a := NewTokenAuthenticator("tkn")
	mw := RequireAuth(a)
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("auth GET: status %d, want 200", w.Result().StatusCode)
	}
}

func TestRequireAuthRedirectsUnauthenticated(t *testing.T) {
	a := NewTokenAuthenticator("tkn")
	mw := RequireAuth(a)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("no auth GET: status %d, want 303", w.Result().StatusCode)
	}
}
