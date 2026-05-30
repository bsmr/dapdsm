package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/auth"
)

func TestBackupsListRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/backups?bg=mybg", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "mybg") {
		t.Errorf("missing bg name in render: %s", w.Body.String())
	}
}

func TestBackupsCreateRoute(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"bg": {"mybg"}, "name": {"weekly"}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/backups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBackupsRestoreRouteRequiresConfirm(t *testing.T) {
	srv := newTestServer(t)
	body := url.Values{"key": {"anything"}}.Encode()
	req := httptest.NewRequest("POST", "/host/vm-a/backups/restore", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://127.0.0.1:8765")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 without confirm", w.Code)
	}
}
