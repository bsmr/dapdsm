// internal/pkg/dunemgr/server/notfound_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/domain/auth"
)

func TestNotFoundRendersFriendlyPage(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/no/such/path", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no page at") {
		t.Errorf("body missing friendly text: %s", w.Body.String())
	}
}

func TestExistingRoutesStillWinOverCatchAll(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest("GET", "/hosts", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Errorf("/hosts hit the 404 catch-all (status=404), should render the sidebar")
	}
}
