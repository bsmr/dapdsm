package schedule

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/domain/lifecycle"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// okRunner is an ssh.Runner that reports success for both Run and
// RunWithStdin (kubectl exec / battlegroup verbs).
// calls captures all argv tokens (name + args) so tests can assert on
// subcommands that the ssh.Client passes as arguments to the "ssh" binary.
type okRunner struct{ calls []string }

func (r *okRunner) Run(_ context.Context, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, name)
	r.calls = append(r.calls, args...)
	// Return the mq-game pod in ns/pod format (-A path) when the MQ pod list is
	// queried so that broadcast.PublishInner can pick the game broker.
	// After the shell-quoting fix the args contain a single quoted remote-command
	// token; match against it directly.
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "'get'") && strings.Contains(joined, "'pods'") && strings.Contains(joined, "messagequeue") {
		return ssh.Result{Stdout: "funcom-seabass-x/seabass-mq-game-sts-0\n", ExitCode: 0}, nil
	}
	return ssh.Result{Stdout: "ok", ExitCode: 0}, nil
}
func (r *okRunner) RunWithStdin(_ context.Context, _ []byte, name string, args ...string) (ssh.Result, error) {
	r.calls = append(r.calls, name)
	r.calls = append(r.calls, args...)
	return ssh.Result{Stdout: "publish=ok", ExitCode: 0}, nil
}

func newTestManager(t *testing.T) (*Manager, *store.Store, *okRunner) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	rr := &okRunner{}
	cli := &ssh.Client{Runner: rr}
	m := NewManager(
		&broadcast.Runner{Exec: cli, Store: s},
		&lifecycle.Runner{SSH: cli, Store: s},
		s, nil,
	)
	return m, s, rr
}

func TestSchedulePersistsAndAnnounces(t *testing.T) {
	m, s, _ := newTestManager(t)
	err := m.Schedule(context.Background(), "local", "vm-a", Request{
		Kind: "Restart", LeadSecs: 600, Action: lifecycle.ActionStop,
		ShutdownDurationS: 30, BroadcastFrequency: 60, BroadcastDuration: 10,
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	rec, err := s.GetSchedule("vm-a")
	if err != nil {
		t.Fatalf("no pending record: %v", err)
	}
	if rec.AtUnix-rec.NowUnix != 600 || rec.Action != "stop" {
		t.Errorf("rec=%+v", rec)
	}
	if _, ok := m.Pending("vm-a"); !ok {
		t.Error("Pending(vm-a) = false, want a live timer")
	}
}

func TestScheduleRejectsNonStopRestart(t *testing.T) {
	m, _, _ := newTestManager(t)
	for _, a := range []lifecycle.Action{lifecycle.ActionStart, lifecycle.ActionUpdate} {
		if err := m.Schedule(context.Background(), "local", "vm-a", Request{
			Kind: "Restart", LeadSecs: 60, Action: a,
		}); err == nil {
			t.Errorf("Schedule(action=%s): err=nil, want non-nil", a)
		}
	}
}

func TestScheduleRejectsNonPositiveLead(t *testing.T) {
	m, _, _ := newTestManager(t)
	if err := m.Schedule(context.Background(), "local", "vm-a", Request{
		Kind: "Restart", LeadSecs: 0, Action: lifecycle.ActionStop,
	}); err == nil {
		t.Error("Schedule(lead=0): err=nil, want non-nil")
	}
}

func TestCancelStopsAndDeletes(t *testing.T) {
	m, s, _ := newTestManager(t)
	_ = m.Schedule(context.Background(), "local", "vm-a", Request{
		Kind: "Restart", LeadSecs: 600, Action: lifecycle.ActionStop,
	})
	if err := m.Cancel(context.Background(), "local", "vm-a"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, ok := m.Pending("vm-a"); ok {
		t.Error("Pending(vm-a) = true after Cancel")
	}
	if _, err := s.GetSchedule("vm-a"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("record still present after Cancel: %v", err)
	}
}

func TestExecuteRunsLifecycleAndCleansUp(t *testing.T) {
	m, s, rr := newTestManager(t)
	_ = s.PutSchedule(store.ScheduledShutdown{Host: "vm-a", Action: "stop", AtUnix: 1})
	m.executeShutdown(context.Background(), "vm-a")
	if _, err := s.GetSchedule("vm-a"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("record not cleaned after execute: %v", err)
	}
	// After the shell-quoting fix the binary path is embedded inside a quoted
	// remote-command token (e.g. "'/home/dune/.dune/bin/battlegroup' 'stop'"),
	// so we cannot match it as an exact element. Check for containment instead.
	found := false
	for _, c := range rr.calls {
		if strings.Contains(c, lifecycle.DefaultBattlegroupBin) {
			found = true
		}
	}
	if !found {
		t.Errorf("execute did not invoke the battlegroup binary; calls=%v", rr.calls)
	}
}

func TestRearmExecutesPastDueAndArmsFuture(t *testing.T) {
	m, s, _ := newTestManager(t)
	now := time.Now().Unix()
	_ = s.PutSchedule(store.ScheduledShutdown{Host: "past", Action: "stop", AtUnix: now - 10})
	_ = s.PutSchedule(store.ScheduledShutdown{Host: "future", Action: "stop", AtUnix: now + 600})

	m.Rearm(context.Background())

	// Past-due was executed: its record is gone.
	if _, err := s.GetSchedule("past"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("past-due record not cleaned by Rearm: %v", err)
	}
	// Future was re-armed: a live timer + its record remain.
	if _, ok := m.Pending("future"); !ok {
		t.Error("future schedule not armed by Rearm")
	}
}
