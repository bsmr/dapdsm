package publicip

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPResolver_ReturnsTrimmedIPv4(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "  203.0.113.42  \n")
	}))
	defer srv.Close()
	r := &HTTPResolver{URL: srv.URL}
	got, err := r.Resolve(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "203.0.113.42" {
		t.Errorf("got %q, want %q", got, "203.0.113.42")
	}
}

func TestHTTPResolver_RejectsHTMLBodies(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html>service unavailable</html>")
	}))
	defer srv.Close()
	r := &HTTPResolver{URL: srv.URL}
	_, err := r.Resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "valid IP") {
		t.Errorf("err = %v, want substring 'valid IP'", err)
	}
}

func TestHTTPResolver_RejectsIPv6(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "2001:db8::1\n")
	}))
	defer srv.Close()
	r := &HTTPResolver{URL: srv.URL}
	_, err := r.Resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "IPv4") {
		t.Errorf("err = %v, want substring 'IPv4'", err)
	}
}

func TestHTTPResolver_NonOKStatusIsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	r := &HTTPResolver{URL: srv.URL}
	_, err := r.Resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Errorf("err = %v, want substring 'status 503'", err)
	}
}
