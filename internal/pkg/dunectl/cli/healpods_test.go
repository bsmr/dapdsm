package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// healPodsRunner records delete-pod calls; ns-lookup returns a canned name.
type healPodsRunner struct {
	deletedPods []string
}

func (r *healPodsRunner) Get(_ context.Context, args ...string) ([]byte, error) {
	if len(args) >= 1 && args[0] == "ns" {
		return []byte("funcom-seabass-sh-deadbeef\n"), nil
	}
	return nil, errors.New("unexpected Get " + strings.Join(args, " "))
}
func (r *healPodsRunner) Patch(context.Context, string, string, string, string, string) error {
	return nil
}
func (r *healPodsRunner) DeletePods(context.Context, string, ...string) error { return nil }
func (r *healPodsRunner) Exec(_ context.Context, _, _ string, command ...string) ([]byte, error) {
	// heal-stuck-pods uses kubectl directly via runner.Exec since the
	// Runner interface only exposes DeletePods by label-selector and we
	// need name-targeted deletes here.
	joined := strings.Join(command, " ")
	if strings.Contains(joined, "kubectl delete pod") {
		fields := strings.Fields(joined)
		for i, f := range fields {
			if f == "pod" && i+1 < len(fields) {
				r.deletedPods = append(r.deletedPods, fields[i+1])
			}
		}
	}
	return nil, nil
}
func (r *healPodsRunner) ExecPiped(context.Context, string, string, []byte, ...string) ([]byte, error) {
	return nil, nil
}

func canned(pods ...podStatus) func(context.Context, kubeRunnerStub, string) ([]podStatus, error) {
	return func(context.Context, kubeRunnerStub, string) ([]podStatus, error) {
		return pods, nil
	}
}

func TestHealStuckPods_DryRunByDefault(t *testing.T) {
	t.Parallel()
	r := &healPodsRunner{}
	deps := healPodsDeps{
		runner: r,
		listPods: func(_ context.Context, _ kubeRunnerStub, _ string) ([]podStatus, error) {
			return []podStatus{
				{Name: "sh-x-sg-overmap-pod-2", Phase: "Running", Ready: false, AgeSeconds: 900},
				{Name: "sh-x-db-dbdepl-util-abc", Phase: "Failed", Ready: false, AgeSeconds: 60},
			}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := runHealStuckPods(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.deletedPods) != 0 {
		t.Errorf("dry-run mode must not delete; got %v", r.deletedPods)
	}
	for _, must := range []string{"sh-x-sg-overmap-pod-2", "sh-x-db-dbdepl-util-abc", "dry-run"} {
		if !strings.Contains(stdout.String(), must) {
			t.Errorf("stdout missing %q\n  got: %s", must, stdout.String())
		}
	}
}

func TestHealStuckPods_ApplyDeletesBothCategories(t *testing.T) {
	t.Parallel()
	r := &healPodsRunner{}
	deps := healPodsDeps{
		runner: r,
		listPods: func(_ context.Context, _ kubeRunnerStub, _ string) ([]podStatus, error) {
			return []podStatus{
				{Name: "sh-x-sg-overmap-pod-2", Phase: "Running", Ready: false, AgeSeconds: 900},
				{Name: "sh-x-db-dbdepl-util-abc", Phase: "Failed", Ready: false, AgeSeconds: 60},
			}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := runHealStuckPods(context.Background(), []string{"--apply"}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.deletedPods) != 2 {
		t.Errorf("--apply: expected 2 deletes, got %v", r.deletedPods)
	}
}

func TestHealStuckPods_SkipsHealthyPods(t *testing.T) {
	t.Parallel()
	r := &healPodsRunner{}
	deps := healPodsDeps{
		runner: r,
		listPods: func(_ context.Context, _ kubeRunnerStub, _ string) ([]podStatus, error) {
			return []podStatus{
				{Name: "sh-x-sg-overmap-pod-2", Phase: "Running", Ready: true, AgeSeconds: 900},
			}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := runHealStuckPods(context.Background(), []string{"--apply"}, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.deletedPods) != 0 {
		t.Errorf("healthy pod must not be deleted; got %v", r.deletedPods)
	}
	if !strings.Contains(stdout.String(), "no stuck pods") {
		t.Errorf("stdout missing 'no stuck pods': %q", stdout.String())
	}
}

func TestHealStuckPods_HonoursThresholdMinutes(t *testing.T) {
	t.Parallel()
	r := &healPodsRunner{}
	deps := healPodsDeps{
		runner: r,
		listPods: func(_ context.Context, _ kubeRunnerStub, _ string) ([]podStatus, error) {
			return []podStatus{
				// Below the 5-min threshold → must NOT be flagged.
				{Name: "sh-x-sg-overmap-pod-2", Phase: "Running", Ready: false, AgeSeconds: 60},
			}, nil
		},
	}
	var stdout, stderr bytes.Buffer
	if err := runHealStuckPods(context.Background(),
		[]string{"--apply", "--threshold-minutes", "5"},
		&stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(r.deletedPods) != 0 {
		t.Errorf("pod below threshold must not be deleted; got %v", r.deletedPods)
	}
}

// silence the unused canned helper — it can be promoted to a public
// fixture later when more tests need the same shape.
var _ = canned
