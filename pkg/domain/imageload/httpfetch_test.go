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

// fakeKC records exec argv and lists one importer pod.
type fakeKC struct{ execArgs [][]string }

func (f *fakeKC) Run(_ context.Context, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "get" {
		return "importer-pod-0", nil // pod-name listing
	}
	f.execArgs = append(f.execArgs, args)
	return "", nil
}

func (f *fakeKC) Stdin(_ context.Context, _ []byte, args ...string) (string, error) {
	f.execArgs = append(f.execArgs, args)
	return "", nil
}

func TestLoadViaHTTP_StartsServerCurlsImportsStops(t *testing.T) {
	jp := &fakeJump{stdout: "4242\n"} // nohup server PID
	kc := &fakeKC{}
	_, err := LoadViaHTTP(context.Background(), kc, jp, HTTPOptions{
		Options:       Options{Namespace: "ds-arrakis-imageload", CtrPath: "/host/ctr"},
		TarPathOnJump: "/home/dune/depot/prod/battlegroup/server.tar",
		ServeDir:      "/home/dune/depot/prod/battlegroup",
		ServePort:     8080,
		JumpAddr:      "192.168.33.2:8080",
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
	// The pod exec curls the JumpAddr and pipes into ctr import.
	var curled bool
	for _, a := range kc.execArgs {
		joined := strings.Join(a, " ")
		if strings.Contains(joined, "curl") && strings.Contains(joined, "192.168.33.2:8080") &&
			strings.Contains(joined, "images import -") {
			curled = true
		}
	}
	if !curled {
		t.Fatalf("importer pod did not curl|ctr import: %v", kc.execArgs)
	}
}
