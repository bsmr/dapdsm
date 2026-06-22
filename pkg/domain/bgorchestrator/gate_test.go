package bgorchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
)

// seqGetter returns each canned CR in turn, repeating the last forever.
type seqGetter struct {
	crs  [][]byte
	n    int
	errs error
}

func (s *seqGetter) Get(ctx context.Context, args ...string) ([]byte, error) {
	if s.errs != nil {
		return nil, s.errs
	}
	cr := s.crs[s.n]
	if s.n < len(s.crs)-1 {
		s.n++
	}
	return cr, nil
}

func crReady(ready bool) []byte {
	if ready {
		return []byte(`{"status":{"servers":[{"partitionMap":"m","ready":true}]}}`)
	}
	return []byte(`{"status":{"servers":[{"partitionMap":"m","ready":false}]}}`)
}

func parse(s string) (battlegroup.Status, error) {
	return battlegroup.ParseStatus([]byte(s))
}

func TestWaitForReadyReturnsWhenPredicateHolds(t *testing.T) {
	g := &seqGetter{crs: [][]byte{crReady(false), crReady(false), crReady(true)}}
	err := WaitForPhase(context.Background(), g, "ns", "x", Ready, time.Millisecond, time.Second)
	if err != nil {
		t.Fatalf("want nil once ready, got %v", err)
	}
}

func TestWaitForPhaseTimesOut(t *testing.T) {
	g := &seqGetter{crs: [][]byte{crReady(false)}} // never becomes ready
	err := WaitForPhase(context.Background(), g, "ns", "x", Ready, time.Millisecond, 20*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
}

func TestStoppedPredicate(t *testing.T) {
	none, _ := parse(`{"status":{"servers":[]}}`)
	if !Stopped.OK(none) {
		t.Fatal("empty servers should be Stopped")
	}
	up, _ := parse(`{"status":{"servers":[{"ready":true}]}}`)
	if Stopped.OK(up) {
		t.Fatal("a ready server means not Stopped")
	}
}
