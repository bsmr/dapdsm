package command

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

// itemEditFake implements itemMutator for unit tests.
type itemEditFake struct {
	owner    string
	offline  bool
	affected int64
	stackSet int64
	qualSet  int64
	deleted  bool
}

func (f *itemEditFake) ItemOwnerFLS(_ context.Context, _ string, _ int64) (string, error) {
	return f.owner, nil
}
func (f *itemEditFake) IsPlayerOffline(_ context.Context, _, _ string) (bool, error) {
	return f.offline, nil
}
func (f *itemEditFake) SetItemStack(_ context.Context, _ string, _, stack int64) (int64, error) {
	f.stackSet = stack
	return f.affected, nil
}
func (f *itemEditFake) SetItemQuality(_ context.Context, _ string, _, q int64) (int64, error) {
	f.qualSet = q
	return f.affected, nil
}
func (f *itemEditFake) DeleteItem(_ context.Context, _ string, _ int64) (int64, error) {
	f.deleted = true
	return f.affected, nil
}

func TestApplyItemStackOfflineGate(t *testing.T) {
	// online without force → ErrItemOwnerOnline, no mutation
	online := &itemEditFake{owner: "X", offline: false, affected: 1}
	if err := ApplyItemStack(context.Background(), online, nil, "vm-a", 1, 5, false); !errors.Is(err, ErrItemOwnerOnline) {
		t.Fatalf("online without force: err=%v want ErrItemOwnerOnline", err)
	}
	if online.stackSet != 0 {
		t.Fatalf("must not apply when gated: %d", online.stackSet)
	}

	// offline → apply succeeds
	off := &itemEditFake{owner: "X", offline: true, affected: 1}
	if err := ApplyItemStack(context.Background(), off, nil, "vm-a", 1, 5, false); err != nil {
		t.Fatalf("offline apply: err=%v", err)
	}
	if off.stackSet != 5 {
		t.Fatalf("stack not applied: %d", off.stackSet)
	}

	// unknown owner (empty string) → ErrItemOwnerUnknown
	unk := &itemEditFake{owner: "", offline: true, affected: 1}
	if err := ApplyItemStack(context.Background(), unk, nil, "vm-a", 1, 5, false); !errors.Is(err, ErrItemOwnerUnknown) {
		t.Fatalf("unknown owner: err=%v want ErrItemOwnerUnknown", err)
	}

	// force bypasses the gate even when online
	f := &itemEditFake{owner: "X", offline: false, affected: 1}
	if err := ApplyItemStack(context.Background(), f, nil, "vm-a", 1, 9, true); err != nil {
		t.Fatalf("force apply: err=%v", err)
	}
	if f.stackSet != 9 {
		t.Fatalf("force did not apply: %d", f.stackSet)
	}
}

func TestApplyItemDeleteAndQuality(t *testing.T) {
	// delete offline owner
	d := &itemEditFake{owner: "X", offline: true, affected: 1}
	if err := ApplyItemDelete(context.Background(), d, nil, "vm-a", 1, false); err != nil || !d.deleted {
		t.Fatalf("delete: err=%v deleted=%v", err, d.deleted)
	}

	// quality offline owner
	q := &itemEditFake{owner: "X", offline: true, affected: 1}
	if err := ApplyItemQuality(context.Background(), q, nil, "vm-a", 1, 5, false); err != nil || q.qualSet != 5 {
		t.Fatalf("quality: err=%v qualSet=%d", err, q.qualSet)
	}

	// no matching row → plain error (not a gate error)
	nomatch := &itemEditFake{owner: "X", offline: true, affected: 0}
	if err := ApplyItemDelete(context.Background(), nomatch, nil, "vm-a", 1, false); err == nil || errors.Is(err, ErrItemOwnerOnline) {
		t.Fatalf("no-match delete should be a plain error, got %v", err)
	}
}

func TestItemUsageErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if err := itemCmd(context.Background(), nil, []string{"vm-a"}, &out, &errb); !errors.Is(err, ErrUsage) {
		t.Fatalf("missing sub: err=%v want ErrUsage", err)
	}
	if err := itemCmd(context.Background(), nil, []string{"vm-a", "set"}, &out, &errb); !errors.Is(err, ErrUsage) {
		t.Fatalf("set missing id: err=%v want ErrUsage", err)
	}
	if err := itemCmd(context.Background(), nil, []string{"vm-a", "frob", "1"}, &out, &errb); !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown sub: err=%v want ErrUsage", err)
	}
}

func TestItemSetRequiresField(t *testing.T) {
	var out, errb bytes.Buffer
	err := itemCmd(context.Background(), nil, []string{"vm-a", "set", "8841", "--confirm"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("set without field: err=%v want ErrUsage", err)
	}
}

func TestItemSetRejectsNegative(t *testing.T) {
	var out, errb bytes.Buffer
	err := itemCmd(context.Background(), nil, []string{"vm-a", "set", "8841", "--qty", "-5", "--confirm"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("negative qty: err=%v want ErrUsage", err)
	}
}

func TestItemSpecRegistered(t *testing.T) {
	if !Known("item") {
		t.Fatal("item verb not registered")
	}
	s, ok := SpecFor("item")
	if !ok || !s.FirstArgIsHost() {
		t.Fatalf("item spec missing or not host-first: %+v", s)
	}
}
