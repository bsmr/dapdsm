package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeRunner records Patch invocations and serves a canned BattleGroup
// CR for Get(battlegroup ...).
type fakeRunner struct {
	cr       []byte
	nodeIP   string // value returned for Get(nodes ...); empty means "no ExternalIP"
	patchErr error

	patchCalls []recordedPatch
}

type recordedPatch struct {
	resource, name, namespace, patchType, payload string
}

func (f *fakeRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	if len(args) >= 1 && args[0] == "battlegroup" {
		return f.cr, nil
	}
	if len(args) >= 1 && args[0] == "nodes" {
		return []byte(f.nodeIP), nil
	}
	return nil, errors.New("fakeRunner: unexpected get args")
}

func (f *fakeRunner) Patch(_ context.Context, resource, name, namespace, patchType, payload string) error {
	f.patchCalls = append(f.patchCalls, recordedPatch{resource, name, namespace, patchType, payload})
	return f.patchErr
}

func (f *fakeRunner) DeletePods(context.Context, string, ...string) error { return nil }

func (f *fakeRunner) Exec(context.Context, string, string, ...string) ([]byte, error) {
	return nil, nil
}
func (f *fakeRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

type fakeResolver struct {
	ip  string
	err error
}

func (f fakeResolver) Resolve(context.Context) (string, error) { return f.ip, f.err }

const minimalCR = `{
  "spec": {
    "utilities": {
      "director": {
        "spec": {
          "envVars": [
            {"name": "HOST_DATACENTER_IP_ADDRESS", "value": "127.0.0.1"}
          ]
        }
      }
    },
    "serverGroup": {
      "template": {
        "spec": {
          "sets": [
            {"schedulerName": "memory-focused-scheduler"}
          ]
        }
      }
    }
  }
}`

func TestPatchBattlegroup_PrefersNodeExternalIPOverResolver(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{cr: []byte(minimalCR), nodeIP: "192.0.2.151"}
	resolverErr := errors.New("must not be called: node IP wins")
	deps := patchBgDeps{runner: r, resolver: fakeResolver{err: resolverErr}}
	var stdout, stderr bytes.Buffer
	if err := patchBattlegroup(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "player IP:   192.0.2.151") {
		t.Errorf("stdout missing node-ExternalIP: %q", stdout.String())
	}

	var firstOps []map[string]any
	if err := json.Unmarshal([]byte(r.patchCalls[0].payload), &firstOps); err != nil {
		t.Fatalf("first patch payload is not JSON: %v", err)
	}
	if len(firstOps) != 1 || firstOps[0]["value"] != "192.0.2.151" {
		t.Errorf("first patch ops = %v, want one op with value 192.0.2.151", firstOps)
	}
}

func TestPatchBattlegroup_FallsBackToResolverWhenNodeHasNoExternalIP(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{cr: []byte(minimalCR), nodeIP: ""}
	deps := patchBgDeps{runner: r, resolver: fakeResolver{ip: "203.0.113.42"}}
	var stdout, stderr bytes.Buffer
	if err := patchBattlegroup(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.patchCalls) != 2 {
		t.Fatalf("patch calls = %d, want 2 (host IP + scheduler removal)", len(r.patchCalls))
	}
	if !strings.Contains(stdout.String(), "namespace:   funcom-seabass-sh-deadbeef") {
		t.Errorf("stdout missing detected namespace: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "battlegroup: sh-deadbeef") {
		t.Errorf("stdout missing derived bg name: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "player IP:   203.0.113.42") {
		t.Errorf("stdout missing resolved IP: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "applied 1 host-IP op(s), 0 host-ID op(s), and 1 scheduler-removal op(s)") {
		t.Errorf("stdout missing summary: %q", stdout.String())
	}

	var firstOps []map[string]any
	if err := json.Unmarshal([]byte(r.patchCalls[0].payload), &firstOps); err != nil {
		t.Fatalf("first patch payload is not JSON: %v", err)
	}
	if len(firstOps) != 1 || firstOps[0]["value"] != "203.0.113.42" {
		t.Errorf("first patch ops = %v, want one op with value 203.0.113.42", firstOps)
	}
}

func TestPatchBattlegroup_IdempotentWhenNothingToDo(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{cr: []byte(`{"spec":{}}`)}
	deps := patchBgDeps{runner: r, resolver: fakeResolver{ip: "1.2.3.4"}}
	var stdout, stderr bytes.Buffer
	if err := patchBattlegroup(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.patchCalls) != 0 {
		t.Errorf("patch calls = %d, want 0 (nothing to change)", len(r.patchCalls))
	}
	if !strings.Contains(stdout.String(), "applied 0 host-IP op(s), 0 host-ID op(s), and 0 scheduler-removal op(s)") {
		t.Errorf("stdout = %q, want zero-op summary", stdout.String())
	}
}

func TestPatchBattlegroup_FlagIPSkipsResolver(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{cr: []byte(minimalCR)}
	resolverErr := errors.New("must not be called")
	deps := patchBgDeps{runner: r, resolver: fakeResolver{err: resolverErr}}
	var stdout, stderr bytes.Buffer
	err := patchBattlegroup(
		context.Background(),
		[]string{"--ip", "198.51.100.7", "--namespace", "funcom-seabass-sh-deadbeef", "--bg-name", "sh-deadbeef"},
		&stdout, &stderr, deps,
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stdout.String(), "player IP:   198.51.100.7") {
		t.Errorf("stdout = %q, want flag-supplied IP", stdout.String())
	}
}

func TestPatchBattlegroup_RejectsUnknownFlag(t *testing.T) {
	t.Parallel()
	deps := patchBgDeps{runner: &fakeRunner{}, resolver: fakeResolver{}}
	var stdout, stderr bytes.Buffer
	err := patchBattlegroup(context.Background(), []string{"--no-such-flag"}, &stdout, &stderr, deps)
	if !errors.Is(err, ErrUsage) {
		t.Errorf("err = %v, want errors.Is(err, ErrUsage)", err)
	}
}
