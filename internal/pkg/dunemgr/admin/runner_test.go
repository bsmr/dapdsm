package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/mq"
)

// recordingPublisher implements the publisher interface for test purposes.
// It captures the PublishInner call and returns a preset result.
type recordingPublisher struct {
	calls  []publishCall
	result *mq.Result
	err    error
}

type publishCall struct {
	operator, host, action, subject, innerJSON, label string
}

func (r *recordingPublisher) PublishInner(
	ctx context.Context,
	operator, host, action, subject, innerJSON, label string,
) (*mq.Result, error) {
	r.calls = append(r.calls, publishCall{
		operator:  operator,
		host:      host,
		action:    action,
		subject:   subject,
		innerJSON: innerJSON,
		label:     label,
	})
	if r.result == nil {
		return &mq.Result{OK: true, RawOutput: "publish=ok"}, r.err
	}
	return r.result, r.err
}

// newRunner builds a Runner with a fresh recordingPublisher.
func newRunner() (*Runner, *recordingPublisher) {
	rp := &recordingPublisher{}
	return &Runner{MQ: rp}, rp
}

// --- gating tests ---

func TestRunner_KickWithoutConfirm(t *testing.T) {
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "kick", "player-x",
		map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for kick without --confirm")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("error missing 'destructive': %v", err)
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

func TestRunner_CleanWithoutConfirm(t *testing.T) {
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "clean", "player-x",
		map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for clean without --confirm")
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

func TestRunner_ResetWithoutConfirm(t *testing.T) {
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "reset", "player-x",
		map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for reset without --confirm")
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

func TestRunner_WildcardWithoutConfirm(t *testing.T) {
	r, rp := newRunner()
	// item allows *, but confirm must be true for wildcard.
	// Gating check runs before Build so item id does not need to be in catalog.
	_, err := r.Run(context.Background(), "op", "vm-a", "item", "*",
		map[string]string{"ItemName": "T6_Augment_Acuracy1"}, false)
	if err == nil {
		t.Fatal("expected error for wildcard without --confirm")
	}
	if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("error missing 'wildcard': %v", err)
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

func TestRunner_VehicleWildcardRejected(t *testing.T) {
	// vehicle has allowAll=false; * must be rejected even with confirm.
	// Gating check runs before Build so ids are irrelevant here.
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "vehicle", "*",
		map[string]string{
			"ClassName":    "Sandbike",
			"TemplateName": "T1_ExtraSeat",
			"X":            "0", "Y": "0", "Z": "0",
		}, true)
	if err == nil {
		t.Fatal("expected error for vehicle with wildcard player")
	}
	if !strings.Contains(err.Error(), "wildcard") && !strings.Contains(err.Error(), "allowAll") && !strings.Contains(err.Error(), "does not support") {
		t.Errorf("unexpected error text: %v", err)
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

func TestRunner_UnknownVerb(t *testing.T) {
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "notaverb", "player-x",
		map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for unknown verb")
	}
	if len(rp.calls) != 0 {
		t.Errorf("PublishInner must not be called: got %d calls", len(rp.calls))
	}
}

// --- happy path tests ---

func TestRunner_KickHappy(t *testing.T) {
	r, rp := newRunner()
	res, err := r.Run(context.Background(), "op", "vm-a", "kick", "player-x",
		map[string]string{}, true)
	if err != nil {
		t.Fatalf("kick --confirm: %v", err)
	}
	if !res.OK {
		t.Errorf("OK=false")
	}
	if len(rp.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(rp.calls))
	}
	c := rp.calls[0]
	if c.action != "admin.kick" {
		t.Errorf("action=%q, want admin.kick", c.action)
	}
	if c.host != "vm-a" {
		t.Errorf("host=%q, want vm-a", c.host)
	}
	if c.label != "kick" {
		t.Errorf("label=%q, want kick", c.label)
	}
	// Verify inner JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(c.innerJSON), &m); err != nil {
		t.Fatalf("inner JSON: %v", err)
	}
	if m["ServerCommand"] != "KickPlayer" {
		t.Errorf("ServerCommand=%q", m["ServerCommand"])
	}
	if m["PlayerId"] != "player-x" {
		t.Errorf("PlayerId=%q", m["PlayerId"])
	}
}

func TestRunner_ItemHappy(t *testing.T) {
	// T6_Augment_Acuracy1 is a real item id in the vendored catalog.
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "item", "player-y",
		map[string]string{"ItemName": "T6_Augment_Acuracy1"}, false)
	if err != nil {
		t.Fatalf("item: %v", err)
	}
	if len(rp.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(rp.calls))
	}
	c := rp.calls[0]
	if c.action != "admin.item" {
		t.Errorf("action=%q, want admin.item", c.action)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(c.innerJSON), &m); err != nil {
		t.Fatalf("inner JSON: %v", err)
	}
	if m["ItemName"] != "T6_Augment_Acuracy1" {
		t.Errorf("ItemName=%q", m["ItemName"])
	}
}

func TestRunner_WildcardItemWithConfirm(t *testing.T) {
	// T6_Augment_Acuracy1 is a real item id in the vendored catalog.
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-a", "item", "*",
		map[string]string{"ItemName": "T6_Augment_Acuracy1"}, true)
	if err != nil {
		t.Fatalf("item * --confirm: %v", err)
	}
	if len(rp.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(rp.calls))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rp.calls[0].innerJSON), &m); err != nil {
		t.Fatalf("inner JSON: %v", err)
	}
	if m["PlayerId"] != "*" {
		t.Errorf("PlayerId=%q, want *", m["PlayerId"])
	}
}

func TestRunner_SubjectFormat(t *testing.T) {
	r, rp := newRunner()
	_, err := r.Run(context.Background(), "op", "vm-b", "water", "player-z",
		map[string]string{}, false)
	if err != nil {
		t.Fatalf("water: %v", err)
	}
	if len(rp.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(rp.calls))
	}
	if rp.calls[0].subject != "host=vm-b player=player-z" {
		t.Errorf("subject=%q, want 'host=vm-b player=player-z'", rp.calls[0].subject)
	}
}
