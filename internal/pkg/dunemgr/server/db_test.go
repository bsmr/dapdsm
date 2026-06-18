// internal/pkg/dunemgr/server/db_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/auth"
)

func TestDBSchemaRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/db", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDBExecRoute(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"sql": {"SELECT 1"}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/db/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDBExecRouteRejectsEmptySQL(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"sql": {""}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/db/exec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}
