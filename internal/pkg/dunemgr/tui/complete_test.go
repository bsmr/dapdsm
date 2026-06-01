package tui

import (
	"reflect"
	"testing"
)

func TestSuggestVerbsByPrefix(t *testing.T) {
	got := suggest("b", nil)
	// "backup" and "broadcast" both start with b
	if !contains(got, "backup") || !contains(got, "broadcast") {
		t.Errorf("suggest(\"b\") = %v, want backup+broadcast", got)
	}
}

func TestSuggestHostPosition(t *testing.T) {
	hosts := []string{"vm-a", "vm-b"}
	got := suggest("lifecycle ", hosts) // trailing space → next token (host)
	if !reflect.DeepEqual(got, []string{"vm-a", "vm-b"}) {
		t.Errorf("suggest host pos = %v, want [vm-a vm-b]", got)
	}
}

func TestSuggestSubVerbPosition(t *testing.T) {
	got := suggest("lifecycle vm-a re", nil) // action token "re"
	if !reflect.DeepEqual(got, []string{"restart"}) {
		t.Errorf("suggest action = %v, want [restart]", got)
	}
}

func TestCompleteUniqueInsertsValueAndSpace(t *testing.T) {
	got, _ := complete("lifecycle vm-a re", nil)
	if got != "lifecycle vm-a restart " {
		t.Errorf("complete = %q, want \"lifecycle vm-a restart \"", got)
	}
}

func TestCompleteCommonPrefixOnAmbiguous(t *testing.T) {
	// host verb sub-options add/list/rm/probe; "" → no common prefix beyond ""
	got, cands := complete("host ", nil)
	if got != "host " {
		t.Errorf("ambiguous complete should not change line, got %q", got)
	}
	if len(cands) != 4 {
		t.Errorf("expected 4 host sub candidates, got %v", cands)
	}
}

func TestCompleteFreeformNoCompletion(t *testing.T) {
	got, cands := complete("db vm-a exec sel", nil) // exec's next slot is argFree (sql)
	if got != "db vm-a exec sel" || len(cands) != 0 {
		t.Errorf("freeform should not complete: line=%q cands=%v", got, cands)
	}
}

func TestCompleteUnknownVerbNoCandidates(t *testing.T) {
	_, cands := complete("frob ", nil)
	if len(cands) != 0 {
		t.Errorf("unknown verb args → no candidates, got %v", cands)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
