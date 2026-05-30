package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/auth"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/backup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/lifecycle"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// recordingSSHRunner is a fake ssh.Runner that returns canned success
// results without spawning any real processes. When name == "scp" it
// materialises a dummy file at the last argument (the local destination)
// so that backup.Create can stat it — mirroring the pattern in
// backup/create_test.go::sshFake.
type recordingSSHRunner struct{}

func (r *recordingSSHRunner) Run(_ context.Context, name string, args ...string) (ssh.Result, error) {
	if name == "scp" && len(args) >= 1 {
		dst := args[len(args)-1]
		_ = os.WriteFile(dst, []byte("dummy"), 0o644)
	}
	// discoverDB issues `kubectl get databasedeployment` and expects a
	// four-field jsonpath response: namespace, name, port, superUser.
	if strings.Contains(strings.Join(args, " "), "databasedeployment") {
		return ssh.Result{Stdout: "funcom-seabass-x funcom-dunedb 15432 postgres\n", ExitCode: 0}, nil
	}
	return ssh.Result{Stdout: "started\n", ExitCode: 0}, nil
}

func (r *recordingSSHRunner) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: "publish=ok", ExitCode: 0}, nil
}

func newTestServerWithHub(t *testing.T, hub *sse.Hub) *Server {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	fakeSSH := &ssh.Client{Runner: &recordingSSHRunner{}}
	srv, err := New(Options{
		Token:           "tkn",
		Host:            "127.0.0.1:8765",
		Store:           s,
		SSHClient:       fakeSSH,
		LifecycleRunner: &lifecycle.Runner{SSH: fakeSSH, Store: s},
		BackupRunner:    &backup.Runner{SSH: fakeSSH, Store: s, DataDir: t.TempDir()},
		DBRunner:        &dbquery.Runner{SSH: fakeSSH, Store: s},
		Hub:             hub,
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return newTestServerWithHub(t, sse.NewHub())
}

func TestLoginPageRenders(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/login", nil)
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("/login GET status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `name="token"`) {
		t.Errorf("login body missing token field: %s", w.Body.String())
	}
}

func TestHomeRedirectsUnauthenticated(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Errorf("anonymous / status = %d, want 303", w.Code)
	}
}

func TestHomeServedWhenAuthenticated(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("auth / status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Hello, operator") {
		t.Errorf("home missing greeting; body = %s", w.Body.String())
	}
}

func TestStaticServed(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/static/pico.min.css", nil)
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("static status = %d, want 200", w.Code)
	}
}

func TestHostsSidebarRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/hosts", nil)
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Errorf("anon /hosts: status %d, want 303", w.Code)
	}
}

func TestHostsSidebarServedWhenAuthenticated(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/hosts", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("auth /hosts: status %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no hosts yet") {
		t.Errorf("sidebar should show empty state: %q", w.Body.String())
	}
}

func TestHostDashboardRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/host/vm-a", nil)
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Errorf("anon /host/vm-a: status %d, want 303", w.Code)
	}
}

func TestHostDashboardServed(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/host/vm-a", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	srv.Handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("auth /host/vm-a: status %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "vm-a") {
		t.Errorf("dashboard missing host name: %q", w.Body.String())
	}
}

func TestHomeShowsSidebarNav(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	srv.Handler.ServeHTTP(w, r)
	body := w.Body.String()
	if !strings.Contains(body, `hx-get="/hosts"`) {
		t.Errorf("home is missing the sidebar nav; body=%s", body)
	}
	if strings.Contains(body, "lands in Phase 2") {
		t.Error("home still shows the Phase-1 stub text")
	}
}

func TestLoginHasNoSidebarNav(t *testing.T) {
	srv := newTestServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/login", nil)
	srv.Handler.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), `hx-get="/hosts"`) {
		t.Error("login page must not load the host sidebar (pre-auth)")
	}
}

const sampleKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: TEFTRTY0Q0FEQVRB
    server: https://vm-a.example.org:6443
  name: default
`

// kubeconfigSSH is an ssh.Runner whose Run returns a canned kubeconfig
// (success) or an error (failure), exercising hostpool.Register's
// fetchK3sCA without a real host.
type kubeconfigSSH struct {
	out string
	err error
}

func (k kubeconfigSSH) Run(_ context.Context, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: k.out}, k.err
}
func (kubeconfigSSH) RunWithStdin(_ context.Context, _ []byte, _ string, _ ...string) (ssh.Result, error) {
	return ssh.Result{}, nil
}

func newServerWithSSH(t *testing.T, runner ssh.Runner) (*Server, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	srv, err := New(Options{
		Token:     "tkn",
		Host:      "127.0.0.1:8765",
		Store:     s,
		SSHClient: &ssh.Client{Runner: runner},
		Hub:       sse.NewHub(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, s
}

func authedReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	r.Header.Set("Origin", "http://127.0.0.1:8765")
	return r
}

func TestHostAddFormRenders(t *testing.T) {
	srv, _ := newServerWithSSH(t, kubeconfigSSH{})
	w := httptest.NewRecorder()
	srv.Handler.ServeHTTP(w, authedReq("GET", "/hosts/add", nil))
	body := w.Body.String()
	if !strings.Contains(body, `name="name"`) || !strings.Contains(body, `name="ssh_alias"`) {
		t.Errorf("add form missing inputs; body=%s", body)
	}
}

func TestHostAddSuccessRedirects(t *testing.T) {
	srv, s := newServerWithSSH(t, kubeconfigSSH{out: sampleKubeconfig})
	form := strings.NewReader("name=vm-x&ssh_alias=vm-x")
	w := httptest.NewRecorder()
	req := authedReq("POST", "/hosts/add", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/host/vm-x" {
		t.Errorf("Location = %q, want /host/vm-x", loc)
	}
	if _, err := s.GetHost("vm-x"); err != nil {
		t.Errorf("host not persisted: %v", err)
	}
}

func TestHostAddFailureRerendersForm(t *testing.T) {
	srv, _ := newServerWithSSH(t, kubeconfigSSH{err: errors.New("dial tcp: timeout")})
	form := strings.NewReader("name=vm-y&ssh_alias=vm-y")
	w := httptest.NewRecorder()
	req := authedReq("POST", "/hosts/add", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (re-render)", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "vm-y") {
		t.Errorf("failed add did not preserve the submitted name; body=%s", body)
	}
	if !strings.Contains(body, "fetch K3s CA") {
		t.Errorf("failed add did not surface the error; body=%s", body)
	}
}

func TestLifecyclePartialRendersCachedMaps(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_ = s.PutStatus(store.StatusSnapshot{
		Host: "vm-a", BGState: "RUNNING", PodReady: 1, PodTotal: 1,
		Detail: battlegroup.Status{
			ServerGroupPhase: "Running", DBPhase: "Ready",
			Servers: []battlegroup.ServerStatus{{Map: "Overmap", Phase: "Running", Ready: true}},
		},
	})
	srv, err := New(Options{Token: "tkn", Host: "127.0.0.1:8765", Store: s, Hub: sse.NewHub()})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/host/vm-a/lifecycle/_partial", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "tkn"})
	srv.Handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Overmap") {
		t.Errorf("partial missing map name Overmap; body=%s", body)
	}
	if !strings.Contains(body, "RUNNING") {
		t.Errorf("partial missing BG state; body=%s", body)
	}
}
