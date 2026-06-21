package hostprep

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// kubectlArgs captures the sequence of kubectl invocations for assertion.
type recordingRunner struct {
	calls [][]string // each element is one kubectl args slice
	kube  func(call int, args ...string) (ssh.Result, error)
}

func (r *recordingRunner) Kubectl(_ context.Context, args ...string) (ssh.Result, error) {
	idx := len(r.calls)
	r.calls = append(r.calls, append([]string{}, args...))
	return r.kube(idx, args...)
}

func (r *recordingRunner) OnJump(_ context.Context, _ string, _ ...string) (ssh.Result, error) {
	panic("OnJump must not be called from ControlPlaneTaint")
}

// cpTaintRunner builds a recordingRunner that returns canned kubectl results:
// call 0 → cpOutput (control-plane nodes+taints query)
// call 1 → workerOutput (worker nodes query)
func cpTaintRunner(cpOutput string, cpErr error, workerOutput string) *recordingRunner {
	return &recordingRunner{
		kube: func(call int, args ...string) (ssh.Result, error) {
			switch call {
			case 0:
				return ssh.Result{Stdout: cpOutput}, cpErr
			case 1:
				return ssh.Result{Stdout: workerOutput}, nil
			default:
				panic("unexpected kubectl call")
			}
		},
	}
}

// cpTaintRunnerWorkerErr builds a recordingRunner where call 0 succeeds and
// call 1 (the worker query) returns an error. Used to test Fix 1.
func cpTaintRunnerWorkerErr(cpOutput string, workerErr error) *recordingRunner {
	return &recordingRunner{
		kube: func(call int, args ...string) (ssh.Result, error) {
			switch call {
			case 0:
				return ssh.Result{Stdout: cpOutput}, nil
			case 1:
				return ssh.Result{}, workerErr
			default:
				panic("unexpected kubectl call")
			}
		},
	}
}

// assertQuery verifies that the kubectl call at index i contains the expected
// args substring. This pins the query shape so it cannot silently drift.
func assertQuery(t *testing.T, r *recordingRunner, callIdx int, mustContain ...string) {
	t.Helper()
	if callIdx >= len(r.calls) {
		t.Fatalf("expected kubectl call %d, got %d calls total", callIdx, len(r.calls))
	}
	joined := strings.Join(r.calls[callIdx], " ")
	for _, want := range mustContain {
		if !strings.Contains(joined, want) {
			t.Errorf("kubectl call %d %q does not contain %q", callIdx, joined, want)
		}
	}
}

// TestControlPlaneTaint_MultiNode_AllTainted: multi-node cluster where all CP
// nodes carry NoExecute — expected result is OK=true.
func TestControlPlaneTaint_MultiNode_AllTainted(t *testing.T) {
	// Two CP nodes, each with NoExecute; two worker nodes.
	// New format: effects are space-separated tokens before the ";" separator.
	r := cpTaintRunner(
		"cp-00=NoExecute ;cp-01=NoExecute ;",
		nil,
		"worker-00 worker-01",
	)
	c := ControlPlaneTaint(context.Background(), r)

	if !c.OK {
		t.Errorf("want OK=true when all CP nodes are tainted, got Detail=%q", c.Detail)
	}
	assertQuery(t, r, 0, "-l", "node-role.kubernetes.io/control-plane", "jsonpath")
	assertQuery(t, r, 1, "-l", "!node-role.kubernetes.io/control-plane", "jsonpath")
}

// TestControlPlaneTaint_MultiNode_SomeTainted: one CP node is untainted in a
// multi-node cluster — expected result is OK=false naming the untainted node.
func TestControlPlaneTaint_MultiNode_SomeTainted(t *testing.T) {
	// cp-00 has NoExecute; cp-01 has empty effects (untainted).
	r := cpTaintRunner(
		"cp-00=NoExecute ;cp-01=;",
		nil,
		"worker-00",
	)
	c := ControlPlaneTaint(context.Background(), r)

	if c.OK {
		t.Errorf("want OK=false when a CP node is untainted, got OK=true")
	}
	if !strings.Contains(c.Detail, "cp-01") {
		t.Errorf("Detail should name the untainted node cp-01, got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "CriticalAddonsOnly") {
		t.Errorf("Detail should mention CriticalAddonsOnly hint, got %q", c.Detail)
	}
}

// TestControlPlaneTaint_SingleNode: CP present but no workers — must stay
// schedulable; expected result is OK=true.
func TestControlPlaneTaint_SingleNode(t *testing.T) {
	r := cpTaintRunner(
		"cp-00=;",
		nil,
		"", // no workers
	)
	c := ControlPlaneTaint(context.Background(), r)

	if !c.OK {
		t.Errorf("want OK=true for single-node cluster, got Detail=%q", c.Detail)
	}
	if !strings.Contains(c.Detail, "single-node") {
		t.Errorf("Detail should mention single-node, got %q", c.Detail)
	}
}

// TestControlPlaneTaint_NoCPNodes: no control-plane-labeled nodes — nothing to
// check; expected result is OK=true.
func TestControlPlaneTaint_NoCPNodes(t *testing.T) {
	r := cpTaintRunner(
		"", // empty output → no CP nodes
		nil,
		"worker-00",
	)
	c := ControlPlaneTaint(context.Background(), r)

	if !c.OK {
		t.Errorf("want OK=true when no CP nodes exist, got Detail=%q", c.Detail)
	}
}

// TestControlPlaneTaint_KubectlError: query 1 errors — must return OK=false
// with a descriptive detail and NOT panic.
func TestControlPlaneTaint_KubectlError(t *testing.T) {
	injectedErr := errors.New("connection refused")
	r := &recordingRunner{
		kube: func(call int, args ...string) (ssh.Result, error) {
			if call == 0 {
				return ssh.Result{Stderr: "connection refused"}, injectedErr
			}
			panic("should not reach query 2 on query 1 error")
		},
	}
	c := ControlPlaneTaint(context.Background(), r)

	if c.OK {
		t.Errorf("want OK=false on kubectl error, got OK=true")
	}
	if !strings.Contains(c.Detail, "connection refused") {
		t.Errorf("Detail should contain the error, got %q", c.Detail)
	}
}

// TestControlPlaneTaint_WorkerQueryError: query 1 succeeds (one untainted CP
// node), query 2 returns an error — must return OK=false with a detail
// mentioning "could not query worker nodes" (Fix 1).
func TestControlPlaneTaint_WorkerQueryError(t *testing.T) {
	injectedErr := errors.New("timeout reaching apiserver")
	// cp-00 is present but untainted; the worker query will fail.
	r := cpTaintRunnerWorkerErr("cp-00=;", injectedErr)
	c := ControlPlaneTaint(context.Background(), r)

	if c.OK {
		t.Errorf("want OK=false when worker query errors, got OK=true")
	}
	if !strings.Contains(c.Detail, "could not query worker nodes") {
		t.Errorf("Detail should mention 'could not query worker nodes', got %q", c.Detail)
	}
	// Assert query 2 was actually reached (not short-circuited).
	if len(r.calls) < 2 {
		t.Errorf("expected query 2 to be reached, but only %d call(s) made", len(r.calls))
	}
	assertQuery(t, r, 1, "!node-role.kubernetes.io/control-plane")
}

// TestControlPlaneTaint_PreferNoScheduleOnly: CP node has only PreferNoSchedule
// in a multi-node cluster — must return OK=false because PreferNoSchedule does
// NOT repel workloads (Fix 2 regression guard).
func TestControlPlaneTaint_PreferNoScheduleOnly(t *testing.T) {
	// cp-00 has only PreferNoSchedule (not a hard repelling taint).
	r := cpTaintRunner(
		"cp-00=PreferNoSchedule ;",
		nil,
		"worker-00",
	)
	c := ControlPlaneTaint(context.Background(), r)

	if c.OK {
		t.Errorf("want OK=false when CP node only has PreferNoSchedule, got OK=true")
	}
	if !strings.Contains(c.Detail, "cp-00") {
		t.Errorf("Detail should name the schedulable CP node cp-00, got %q", c.Detail)
	}
}
