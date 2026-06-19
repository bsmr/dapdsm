package cli

import (
	"flag"
	"slices"
	"testing"
)

func newTestFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.Bool("apply", false, "")
	_ = fs.Bool("restart", false, "")
	_ = fs.String("file", "", "")
	_ = fs.String("section", "", "")
	return fs
}

func TestReorderFlagArgs_NoChangeWhenAlreadyOrdered(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	in := []string{"--apply", "--file", "/tmp/x", "key", "value"}
	got := reorderFlagArgs(fs, in)
	if !slices.Equal(got, in) {
		t.Errorf("got %v, want %v", got, in)
	}
}

func TestReorderFlagArgs_TrailingBoolFlagsMovedFront(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	in := []string{"key", "value", "--apply", "--restart"}
	want := []string{"--apply", "--restart", "key", "value"}
	got := reorderFlagArgs(fs, in)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderFlagArgs_StringFlagAndValueTravelTogether(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	in := []string{"key", "value", "--file", "/etc/x", "--apply"}
	want := []string{"--file", "/etc/x", "--apply", "key", "value"}
	got := reorderFlagArgs(fs, in)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderFlagArgs_EqualsFormStaysOneArg(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	in := []string{"key", "value", "--file=/etc/x"}
	want := []string{"--file=/etc/x", "key", "value"}
	got := reorderFlagArgs(fs, in)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderFlagArgs_DoubleDashHaltsClassification(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	in := []string{"--apply", "--", "--not-a-flag", "key"}
	want := []string{"--apply", "--not-a-flag", "key"}
	got := reorderFlagArgs(fs, in)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestReorderFlagArgs_UnknownFlagPassedThrough(t *testing.T) {
	t.Parallel()
	fs := newTestFlagSet()
	// flag.Parse will reject this — reorder must keep it as-is so the
	// error surfaces with the original flag name.
	in := []string{"key", "value", "--bogus"}
	got := reorderFlagArgs(fs, in)
	want := []string{"--bogus", "key", "value"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
