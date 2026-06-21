package imageload

import (
	"context"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

type fakeJump struct {
	cmds   [][]string
	stdout string
}

func (f *fakeJump) OnJump(_ context.Context, name string, args ...string) (ssh.Result, error) {
	f.cmds = append(f.cmds, append([]string{name}, args...))
	return ssh.Result{Stdout: f.stdout}, nil
}

// fakeKC records all Run and Stdin argv; runCalls holds every Run invocation.
type fakeKC struct {
	runCalls [][]string // all Run() invocations (including "get")
	pods     []string   // pod names returned by "get" calls; defaults to ["importer-pod-0"]
}

func (f *fakeKC) Run(_ context.Context, args ...string) (string, error) {
	f.runCalls = append(f.runCalls, args)
	if len(args) > 0 && args[0] == "get" {
		names := f.pods
		if len(names) == 0 {
			names = []string{"importer-pod-0"}
		}
		return strings.Join(names, " "), nil
	}
	return "", nil
}

func (f *fakeKC) Stdin(_ context.Context, _ []byte, args ...string) (string, error) {
	f.runCalls = append(f.runCalls, args)
	return "", nil
}

func TestLoadViaHTTP_StartsServerCurlsImportsStops(t *testing.T) {
	jp := &fakeJump{stdout: "4242\n"} // nohup server PID
	kc := &fakeKC{}
	_, err := LoadViaHTTP(context.Background(), kc, jp, HTTPOptions{
		Options:   Options{Namespace: "ds-arrakis-imageload", CtrPath: "/host/ctr"},
		TarPaths:  []string{"/home/dune/depot/prod/battlegroup/server.tar"},
		ServeDir:  "/home/dune/depot/prod/battlegroup",
		ServePort: 8080,
		JumpAddr:  "192.168.33.2:8080",
	})
	if err != nil {
		t.Fatalf("LoadViaHTTP: %v", err)
	}
	// A server was started and later killed by the captured PID.
	var started, killed bool
	for _, c := range jp.cmds {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "http.server") || strings.Contains(joined, "8080") {
			started = true
		}
		if strings.Contains(joined, "kill") && strings.Contains(joined, "4242") {
			killed = true
		}
	}
	if !started || !killed {
		t.Fatalf("server lifecycle missing (started=%v killed=%v): %v", started, killed, jp.cmds)
	}
	// The pod exec uses wget (not curl) and pipes into ctr import.
	var wgeted bool
	for _, a := range kc.runCalls {
		joined := strings.Join(a, " ")
		if strings.Contains(joined, "wget") && strings.Contains(joined, "192.168.33.2:8080") &&
			strings.Contains(joined, "images import -") {
			wgeted = true
		}
	}
	if !wgeted {
		t.Fatalf("importer pod did not wget|ctr import: %v", kc.runCalls)
	}
}

func TestLoadViaHTTP_WgetAllTars(t *testing.T) {
	kc := &fakeKC{pods: []string{"imp-a", "imp-b"}} // records Run/Stdin calls
	jp := &fakeJump{stdout: "4242\n"}
	opts := HTTPOptions{
		Options:   Options{Namespace: "ns", CtrPath: "/var/lib/rancher/rke2/bin/ctr"},
		TarPaths:  []string{"/d/server.tar", "/d/seabass-director.tar"},
		ServeDir:  "/d", ServePort: 8080, JumpAddr: "192.168.13.5:8080",
	}
	res, err := LoadViaHTTP(context.Background(), kc, jp, opts)
	if err != nil {
		t.Fatalf("LoadViaHTTP: %v", err)
	}
	// 2 tars × 2 pods = 4 import exec calls, all using wget (not curl).
	var execs []string
	for _, c := range kc.runCalls {
		if len(c) >= 1 && c[0] == "exec" {
			execs = append(execs, strings.Join(c, " "))
		}
	}
	if len(execs) != 4 {
		t.Fatalf("want 4 import execs, got %d: %v", len(execs), execs)
	}
	for _, e := range execs {
		if strings.Contains(e, "curl") {
			t.Errorf("import still uses curl: %s", e)
		}
		if !strings.Contains(e, "wget -qO-") {
			t.Errorf("import does not use wget -qO-: %s", e)
		}
	}
	if !strings.Contains(execs[0], "http://192.168.13.5:8080/server.tar") {
		t.Errorf("first import URL wrong: %s", execs[0])
	}
	if got := res.Tars; len(got) != 2 {
		t.Errorf("Result.Tars = %v, want 2 entries", got)
	}
}
