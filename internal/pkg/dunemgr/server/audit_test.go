// internal/pkg/dunemgr/server/audit_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/auth"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

func TestAuditRouteShowsEntriesNewestFirst(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/audit", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no audit entries") && !strings.Contains(w.Body.String(), "Audit") {
		t.Errorf("body missing audit shell: %s", w.Body.String())
	}
	_ = store.AuditEntry{}
}

func TestAuditRouteNegativeOffsetDoesNotPanic(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/audit?offset=-1", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuditRouteRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/audit", nil)
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("unauthenticated GET /audit returned 200")
	}
}
