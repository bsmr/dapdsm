package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner is a minimal Runner used to test the derived helpers.
type fakeRunner struct {
	getResp []byte
	getErr  error
	getArgs [][]string
}

func (f *fakeRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	f.getArgs = append(f.getArgs, args)
	return f.getResp, f.getErr
}

func (f *fakeRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}

func (f *fakeRunner) DeletePods(context.Context, string, ...string) error { return nil }

func (f *fakeRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}

func (f *fakeRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func TestFindBattleGroupNamespace_Found(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{
		getResp: []byte("default\nkube-system\nfuncom-seabass-sh-abcdef\nfuncom-operators\n"),
	}
	got, err := FindBattleGroupNamespace(context.Background(), r)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "funcom-seabass-sh-abcdef" {
		t.Errorf("got %q, want %q", got, "funcom-seabass-sh-abcdef")
	}
}

func TestFindBattleGroupNamespace_NotFound(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{getResp: []byte("default\nkube-system\n")}
	_, err := FindBattleGroupNamespace(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "no funcom-seabass-* namespace") {
		t.Errorf("err = %v, want substring 'no funcom-seabass-* namespace'", err)
	}
}

func TestFindBattleGroupNamespace_PropagatesGetError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("transport down")
	r := &fakeRunner{getErr: sentinel}
	_, err := FindBattleGroupNamespace(context.Background(), r)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
}

func TestNodeExternalIP_ReturnsTrimmedAddress(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{getResp: []byte("192.0.2.151\n")}
	got, err := NodeExternalIP(context.Background(), r)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "192.0.2.151" {
		t.Errorf("got %q, want %q", got, "192.0.2.151")
	}
	if len(r.getArgs) != 1 {
		t.Fatalf("getArgs = %v, want exactly one call", r.getArgs)
	}
	args := r.getArgs[0]
	if len(args) == 0 || args[0] != "nodes" {
		t.Errorf("first arg = %q, want 'nodes'", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "jsonpath") || !strings.Contains(joined, "ExternalIP") {
		t.Errorf("args = %v, want jsonpath query targeting ExternalIP", args)
	}
}

func TestNodeExternalIP_EmptyResultIsError(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{getResp: []byte("\n")}
	_, err := NodeExternalIP(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "no ExternalIP") {
		t.Errorf("err = %v, want substring 'no ExternalIP'", err)
	}
}

func TestNodeExternalIP_PropagatesGetError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("kubectl down")
	r := &fakeRunner{getErr: sentinel}
	_, err := NodeExternalIP(context.Background(), r)
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want errors.Is(err, sentinel)", err)
	}
}

func TestBattleGroupName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"funcom-seabass-sh-abcdef": "sh-abcdef",
		"funcom-seabass-":          "",
		"unrelated-namespace":      "unrelated-namespace",
	}
	for in, want := range cases {
		if got := BattleGroupName(in); got != want {
			t.Errorf("BattleGroupName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCmdRunner_Get_InjectsKubeconfig(t *testing.T) {
	t.Parallel()
	// Bin "echo" echoes the effective argv kubectl would receive, so we
	// can assert what gets passed without a real cluster.
	r := &CmdRunner{Bin: "echo", Kubeconfig: "/run/kc-vm-a"}
	out, err := r.Get(context.Background(), "ns", "--no-headers")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "--kubeconfig=/run/kc-vm-a") {
		t.Errorf("argv %q missing --kubeconfig flag", s)
	}
	if !strings.Contains(s, "get ns --no-headers") {
		t.Errorf("argv %q missing the get verb/args", s)
	}
}

func TestCmdRunner_Get_OmitsKubeconfigWhenUnset(t *testing.T) {
	t.Parallel()
	r := &CmdRunner{Bin: "echo"}
	out, err := r.Get(context.Background(), "ns")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if strings.Contains(string(out), "--kubeconfig") {
		t.Errorf("argv %q must not contain --kubeconfig when unset", string(out))
	}
}

// getterFake implements only kube.Getter.
type getterFake struct{ out string }

func (g getterFake) Get(_ context.Context, _ ...string) ([]byte, error) {
	return []byte(g.out), nil
}

func TestFindBattleGroupNamespaceAcceptsGetter(t *testing.T) {
	t.Parallel()
	var _ Getter = getterFake{} // compile-time: getterFake satisfies Getter
	ns, err := FindBattleGroupNamespace(context.Background(),
		getterFake{out: "default\nfuncom-seabass-x\n"})
	if err != nil {
		t.Fatalf("FindBattleGroupNamespace: %v", err)
	}
	if ns != "funcom-seabass-x" {
		t.Errorf("ns = %q, want funcom-seabass-x", ns)
	}
}

func TestCmdRunner_applyArgs_PrependsKubeconfigAndVerb(t *testing.T) {
	t.Parallel()

	// With kubeconfig set: expect --kubeconfig=<path>, then "apply", then user args.
	c := &CmdRunner{Kubeconfig: "/run/kc"}
	got := c.applyArgs("-f", "/tmp/crds")
	want := []string{"--kubeconfig=/run/kc", "apply", "-f", "/tmp/crds"}
	if len(got) != len(want) {
		t.Fatalf("applyArgs len = %d, want %d; got %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("applyArgs[%d] = %q, want %q", i, got[i], w)
		}
	}

	// Without kubeconfig: "apply" must be the first element.
	c2 := &CmdRunner{}
	got2 := c2.applyArgs("-f", "/tmp/crds")
	want2 := []string{"apply", "-f", "/tmp/crds"}
	if len(got2) != len(want2) {
		t.Fatalf("applyArgs (no kc) len = %d, want %d; got %v", len(got2), len(want2), got2)
	}
	for i, w := range want2 {
		if got2[i] != w {
			t.Errorf("applyArgs (no kc)[%d] = %q, want %q", i, got2[i], w)
		}
	}
}

func TestCmdRunner_Apply_RunsSuccessfully(t *testing.T) {
	t.Parallel()
	// Bin "echo" exits 0, so Apply should return nil and exercise the exec path.
	c := &CmdRunner{Bin: "echo", Kubeconfig: "/run/kc-vm-a"}
	if err := c.Apply(context.Background(), "-f", "/tmp/crds"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}
