package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestBroadcastParsesFlagsAndPublishes(t *testing.T) {
	var gotHost, gotTitle, gotBody string
	var gotDur int
	deps := broadcastDeps{
		publish: func(_ context.Context, _ broadcastTransport, host, title, body string, dur int, _ io.Writer) error {
			gotHost, gotTitle, gotBody, gotDur = host, title, body, dur
			return nil
		},
	}
	var out, errOut bytes.Buffer
	err := runBroadcast(context.Background(),
		[]string{"--title", "Heads up", "--duration", "20", "server restarting soon"},
		&out, &errOut, deps)
	if err != nil {
		t.Fatalf("runBroadcast: %v", err)
	}
	if gotTitle != "Heads up" || gotBody != "server restarting soon" || gotDur != 20 {
		t.Fatalf("bad parse: title=%q body=%q dur=%d", gotTitle, gotBody, gotDur)
	}
	if gotHost != "local" {
		t.Fatalf("default transport should label host %q, got %q", "local", gotHost)
	}
}

func TestBroadcastSSHFlagSelectsRemoteTransport(t *testing.T) {
	var transportLocal bool
	var gotHost string
	deps := broadcastDeps{
		publish: func(_ context.Context, tr broadcastTransport, host, _, _ string, _ int, _ io.Writer) error {
			transportLocal, gotHost = tr.local, host
			return nil
		},
	}
	var out, errOut bytes.Buffer
	if err := runBroadcast(context.Background(), []string{"--ssh", "vm-a", "hi"}, &out, &errOut, deps); err != nil {
		t.Fatalf("runBroadcast: %v", err)
	}
	if transportLocal || gotHost != "vm-a" {
		t.Fatalf("--ssh should select remote transport with host vm-a; local=%v host=%q", transportLocal, gotHost)
	}
}

func TestBroadcastRequiresText(t *testing.T) {
	var out, errOut bytes.Buffer
	err := runBroadcast(context.Background(), []string{"--title", "x"}, &out, &errOut, broadcastDeps{})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("want usage error for missing text, got %v", err)
	}
}
