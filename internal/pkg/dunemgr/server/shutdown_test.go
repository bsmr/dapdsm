// internal/pkg/dunemgr/server/shutdown_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/auth"
)

func TestShutdownScheduleRoute(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"kind": {"Restart"}, "lead": {"600"}, "action": {"stop"}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/shutdown", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestShutdownScheduleRejectsBadAction(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"kind": {"Restart"}, "lead": {"600"}, "action": {"update"}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/shutdown", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway && w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 4xx/502 for bad action", w.Code)
	}
}

func TestShutdownPartialRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/shutdown/_partial", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}
