package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type applyFakeRunner struct {
	deletePodsCalls []applyDeletePodsCall
	deletePodsErr   error
}

type applyDeletePodsCall struct {
	namespace string
	selectors []string
}

func (a *applyFakeRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("kube-system\nfuncom-seabass-sh-deadbeef\n"), nil
	}
	return nil, errors.New("applyFakeRunner: unexpected get args")
}

func (a *applyFakeRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}

func (a *applyFakeRunner) DeletePods(_ context.Context, ns string, selectors ...string) error {
	a.deletePodsCalls = append(a.deletePodsCalls, applyDeletePodsCall{namespace: ns, selectors: selectors})
	return a.deletePodsErr
}

func (a *applyFakeRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (a *applyFakeRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

type writeCall struct {
	path    string
	content []byte
}

func newDeps(ip string) (*applyFakeRunner, *[]writeCall, *int, applyPlayerIPDeps) {
	r := &applyFakeRunner{}
	var writes []writeCall
	restartCalls := 0
	deps := applyPlayerIPDeps{
		runner:   r,
		resolver: fakeResolver{ip: ip},
		writeFile: func(path string, content []byte) error {
			writes = append(writes, writeCall{path: path, content: content})
			return nil
		},
		restartK3s: func(context.Context) error {
			restartCalls++
			return nil
		},
	}
	return r, &writes, &restartCalls, deps
}

func TestSettingsConfBody(t *testing.T) {
	t.Parallel()
	got := string(SettingsConfBody("203.0.113.42"))
	want := "\n\n\n203.0.113.42\n"
	if got != want {
		t.Errorf("SettingsConfBody = %q, want %q", got, want)
	}
}

func TestApplyPlayerIP_HappyPath(t *testing.T) {
	t.Parallel()
	r, writes, restarts, deps := newDeps("203.0.113.42")
	var stdout, stderr bytes.Buffer
	if err := applyPlayerIP(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(*writes) != 1 {
		t.Fatalf("writes = %v, want 1", *writes)
	}
	if (*writes)[0].path != DefaultSettingsConf {
		t.Errorf("write path = %q, want %q", (*writes)[0].path, DefaultSettingsConf)
	}
	if string((*writes)[0].content) != "\n\n\n203.0.113.42\n" {
		t.Errorf("write content = %q, want correct settings.conf", string((*writes)[0].content))
	}
	if *restarts != 1 {
		t.Errorf("restarts = %d, want 1", *restarts)
	}
	if len(r.deletePodsCalls) != 1 {
		t.Fatalf("delete-pods calls = %d, want 1", len(r.deletePodsCalls))
	}
	if r.deletePodsCalls[0].namespace != "funcom-seabass-sh-deadbeef" {
		t.Errorf("delete-pods ns = %q, want %q", r.deletePodsCalls[0].namespace, "funcom-seabass-sh-deadbeef")
	}
	if len(r.deletePodsCalls[0].selectors) != 1 || r.deletePodsCalls[0].selectors[0] != IGWServerSelector {
		t.Errorf("delete-pods selectors = %v, want [%q]", r.deletePodsCalls[0].selectors, IGWServerSelector)
	}
}

func TestApplyPlayerIP_SkipRestartSkipsOnlyRestart(t *testing.T) {
	t.Parallel()
	r, writes, restarts, deps := newDeps("203.0.113.42")
	var stdout, stderr bytes.Buffer
	if err := applyPlayerIP(context.Background(), []string{"--skip-restart"}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if *restarts != 0 {
		t.Errorf("restarts = %d, want 0", *restarts)
	}
	if len(*writes) != 1 {
		t.Errorf("writes = %d, want 1", len(*writes))
	}
	if len(r.deletePodsCalls) != 1 {
		t.Errorf("delete-pods calls = %d, want 1", len(r.deletePodsCalls))
	}
	if !strings.Contains(stdout.String(), "skipping k3s restart") {
		t.Errorf("stdout = %q, want 'skipping k3s restart'", stdout.String())
	}
}

func TestApplyPlayerIP_SkipPodRecreate_DoesNotResolveNamespace(t *testing.T) {
	t.Parallel()
	deps := applyPlayerIPDeps{
		// A runner whose every method errors out — proves the code path
		// neither resolves the namespace nor tries to delete pods when
		// pod recreate is skipped.
		runner:   namespaceLookupShouldFail{},
		resolver: fakeResolver{ip: "203.0.113.42"},
		writeFile: func(string, []byte) error {
			return nil
		},
		restartK3s: func(context.Context) error {
			return nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := applyPlayerIP(context.Background(), []string{"--skip-pod-recreate"}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "skipping pod recreate") {
		t.Errorf("stdout = %q, want 'skipping pod recreate'", stdout.String())
	}
}

// namespaceLookupShouldFail panics if any method other than the no-op ones
// is called. It exists so we can prove the code path skips the namespace
// lookup entirely when pod recreate is disabled.
type namespaceLookupShouldFail struct{}

func (namespaceLookupShouldFail) Get(context.Context, ...string) ([]byte, error) {
	return nil, errors.New("Get must not be called when pod recreate is skipped")
}

func (namespaceLookupShouldFail) Patch(context.Context, string, string, string, string, string) error {
	return errors.New("Patch must not be called")
}

func (namespaceLookupShouldFail) DeletePods(context.Context, string, ...string) error {
	return errors.New("DeletePods must not be called when pod recreate is skipped")
}

func (namespaceLookupShouldFail) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, errors.New("Exec must not be called when pod recreate is skipped")
}
func (namespaceLookupShouldFail) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, errors.New("ExecPiped must not be called when pod recreate is skipped")
}

func TestApplyPlayerIP_ExplicitIPSkipsResolver(t *testing.T) {
	t.Parallel()
	r := &applyFakeRunner{}
	var writes []writeCall
	deps := applyPlayerIPDeps{
		runner:   r,
		resolver: fakeResolver{err: errors.New("must not be called")},
		writeFile: func(path string, content []byte) error {
			writes = append(writes, writeCall{path: path, content: content})
			return nil
		},
		restartK3s: func(context.Context) error { return nil },
	}
	var stdout, stderr bytes.Buffer
	err := applyPlayerIP(
		context.Background(),
		[]string{"--ip", "198.51.100.7", "--namespace", "funcom-seabass-sh-foo"},
		&stdout, &stderr, deps,
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(string(writes[0].content), "198.51.100.7") {
		t.Errorf("write content = %q, want it to contain the flag-supplied IP", string(writes[0].content))
	}
}

func TestApplyPlayerIP_RejectsUnknownFlag(t *testing.T) {
	t.Parallel()
	_, _, _, deps := newDeps("1.2.3.4")
	var stdout, stderr bytes.Buffer
	err := applyPlayerIP(context.Background(), []string{"--no-such-flag"}, &stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
}
