package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/auth"
)

func TestLifecycleRouteHappyPath(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/host/vm-a/lifecycle/start", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "lifecycle start ok") {
		t.Errorf("missing success line: %s", w.Body.String())
	}
}

func TestLifecycleRouteRejectsInvalidAction(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/host/vm-a/lifecycle/destroy", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400; body=%q", w.Code, w.Body.String())
	}
}

func TestLifecycleRouteRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("POST", "/host/vm-a/lifecycle/start", nil)
	// no cookie
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("unauthenticated POST returned 200; want redirect or 401")
	}
}
