package fls

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticateMissingCookie(t *testing.T) {
	a := NewTokenAuthenticator("secret-token")
	r := httptest.NewRequest("GET", "/", nil)
	_, err := a.Authenticate(r)
	if err == nil {
		t.Errorf("no cookie: want error, got nil")
	}
}

func TestAuthenticateWrongCookie(t *testing.T) {
	a := NewTokenAuthenticator("secret-token")
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "wrong"})
	_, err := a.Authenticate(r)
	if err == nil {
		t.Errorf("wrong cookie: want error, got nil")
	}
}

func TestAuthenticateGoodCookie(t *testing.T) {
	a := NewTokenAuthenticator("secret-token")
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: "secret-token"})
	op, err := a.Authenticate(r)
	if err != nil {
		t.Fatalf("good cookie: %v", err)
	}
	if op.ID != "local" {
		t.Errorf("Operator.ID = %q, want local", op.ID)
	}
}

func TestLoginHandlerSetsCookieOnGoodToken(t *testing.T) {
	a := NewTokenAuthenticator("secret-token")
	r := httptest.NewRequest("POST", "/login", nil)
	r.PostForm = map[string][]string{"token": {"secret-token"}}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.LoginHandler().ServeHTTP(w, r)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303 (SeeOther)", resp.StatusCode)
	}
	cookies := resp.Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == SessionCookie && c.Value == "secret-token" {
			found = true
		}
	}
	if !found {
		t.Errorf("session cookie not set; cookies = %v", cookies)
	}
}

func TestLoginHandlerRejectsBadToken(t *testing.T) {
	a := NewTokenAuthenticator("secret-token")
	r := httptest.NewRequest("POST", "/login", nil)
	r.PostForm = map[string][]string{"token": {"wrong"}}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	a.LoginHandler().ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Result().StatusCode)
	}
}
