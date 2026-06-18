// internal/pkg/dunemgr/server/dbextra_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/auth"
)

func TestDBColumnsRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/db/columns?schema=dune&table=player", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDBColumnsRouteRejectsBadIdentifier(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/db/columns?schema=dune'&table=player", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("bad identifier returned 200")
	}
}

func TestDBSlowRoute(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/host/vm-a/db/slow", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status=%d body=%s", w.Code, w.Body.String())
	}
}
