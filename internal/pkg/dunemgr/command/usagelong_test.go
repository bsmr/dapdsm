package command

import (
	"strings"
	"testing"
)

func TestUsageLongPicksMatchingSubVerb(t *testing.T) {
	got := UsageLong([]string{"item", "set"})
	if !strings.Contains(got, "set") || !strings.Contains(got, "--qty") || !strings.Contains(got, "--confirm") {
		t.Fatalf("item set grammar should carry flags, got %q", got)
	}
	if strings.Contains(got, "delete") {
		t.Fatalf("should pick the set line only, got %q", got)
	}
}

func TestUsageLongGiveItem(t *testing.T) {
	got := UsageLong([]string{"give", "item"})
	if !strings.Contains(got, "item") || !strings.Contains(got, "<count>") || !strings.Contains(got, "--quality") {
		t.Fatalf("give item grammar should carry flags, got %q", got)
	}
	if strings.Contains(got, "currency") || strings.Contains(got, "charxp") {
		t.Fatalf("should pick the item line only, got %q", got)
	}
}

func TestUsageLongNoSubVerbReturnsAllGrammar(t *testing.T) {
	got := UsageLong([]string{"item"})
	if !strings.Contains(got, "set") || !strings.Contains(got, "delete") {
		t.Fatalf("bare verb returns all grammar lines, got %q", got)
	}
	if strings.Contains(got, "Edits target") {
		t.Fatalf("prose explanation must be stripped, got %q", got)
	}
}

func TestUsageLongFallsBackToSpecUsage(t *testing.T) {
	s, ok := SpecFor("host")
	if !ok {
		t.Skip("no 'host' spec to test fallback against")
	}
	if got := UsageLong([]string{"host"}); got != s.Usage() {
		t.Fatalf("fallback should equal Spec.Usage(): got %q want %q", got, s.Usage())
	}
}

func TestUsageLongUnknownVerbEmpty(t *testing.T) {
	if got := UsageLong([]string{"nope"}); got != "" {
		t.Fatalf("unknown verb → empty, got %q", got)
	}
	if got := UsageLong(nil); got != "" {
		t.Fatalf("nil argv → empty, got %q", got)
	}
}

func TestUsageLongItemHasNoForeignGrammar(t *testing.T) {
	got := UsageLong([]string{"item"})
	if strings.Contains(got, "player") || strings.Contains(got, "inspect") {
		t.Fatalf("item grammar must not leak the prose 'dunemgr player ... inspect' line, got %q", got)
	}
}

func TestUsageLongNoMatchReturnsAllLines(t *testing.T) {
	got := UsageLong([]string{"item", "zzznotasubverb"})
	if !strings.Contains(got, "set") || !strings.Contains(got, "delete") {
		t.Fatalf("no sub-verb match → all grammar lines, got %q", got)
	}
	if strings.Contains(got, "player") {
		t.Fatalf("must not include foreign grammar, got %q", got)
	}
}

func TestUsageLongGiveCurrency(t *testing.T) {
	got := UsageLong([]string{"give", "currency"})
	if !strings.Contains(got, "currency") || !strings.Contains(got, "<currency-id>") {
		t.Fatalf("give currency grammar must show the currency invocation, got %q", got)
	}
	if strings.Contains(got, "item") || strings.Contains(got, "charxp") {
		t.Fatalf("give currency should pick only the currency line, got %q", got)
	}
}

func TestUsageLongGiveBareListsCurrency(t *testing.T) {
	got := UsageLong([]string{"give"})
	if !strings.Contains(got, "currency") {
		t.Fatalf("bare 'give' must list the currency invocation, got %q", got)
	}
}
