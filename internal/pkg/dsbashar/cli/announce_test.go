package cli

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithAnnounceZeroDelayRunsActionDirectly(t *testing.T) {
	var ran, announced bool
	deps := announceDeps{announce: func(context.Context, time.Duration, string, func(context.Context) error) error {
		announced = true
		return nil
	}}
	err := withAnnounce(context.Background(), 0, "Restart",
		func(context.Context) error { ran = true; return nil }, deps)
	if err != nil || !ran || announced {
		t.Fatalf("zero delay must skip announce: err=%v ran=%v announced=%v", err, ran, announced)
	}
}

func TestWithAnnouncePositiveDelayDelegatesToAnnounce(t *testing.T) {
	var gotDelay time.Duration
	var gotKind string
	deps := announceDeps{announce: func(_ context.Context, d time.Duration, kind string, action func(context.Context) error) error {
		gotDelay, gotKind = d, kind
		return action(context.Background())
	}}
	var ran bool
	err := withAnnounce(context.Background(), 5*time.Minute, "Update",
		func(context.Context) error { ran = true; return nil }, deps)
	if err != nil || !ran || gotDelay != 5*time.Minute || gotKind != "Update" {
		t.Fatalf("delegate failed: err=%v ran=%v delay=%v kind=%q", err, ran, gotDelay, gotKind)
	}
}

func TestWithAnnouncePropagatesActionError(t *testing.T) {
	boom := errors.New("boom")
	deps := announceDeps{announce: func(_ context.Context, _ time.Duration, _ string, action func(context.Context) error) error {
		return action(context.Background())
	}}
	err := withAnnounce(context.Background(), time.Minute, "Restart",
		func(context.Context) error { return boom }, deps)
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}
