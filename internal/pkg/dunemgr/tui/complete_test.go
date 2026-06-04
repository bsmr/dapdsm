package tui

import (
	"reflect"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
)

func TestSuggestVerbsByPrefix(t *testing.T) {
	got := suggest("b", nil, "", nil)
	// "backup" and "broadcast" both start with b
	if !contains(got, "backup") || !contains(got, "broadcast") {
		t.Errorf("suggest(\"b\") = %v, want backup+broadcast", got)
	}
}

func TestSuggestHostPosition(t *testing.T) {
	hosts := []string{"vm-a", "vm-b"}
	got := suggest("lifecycle ", hosts, "", nil) // trailing space → next token (host)
	if !reflect.DeepEqual(got, []string{"vm-a", "vm-b"}) {
		t.Errorf("suggest host pos = %v, want [vm-a vm-b]", got)
	}
}

func TestSuggestSubVerbPosition(t *testing.T) {
	got := suggest("lifecycle vm-a re", nil, "", nil) // action token "re"
	if !reflect.DeepEqual(got, []string{"restart"}) {
		t.Errorf("suggest action = %v, want [restart]", got)
	}
}

func TestCompleteUniqueInsertsValueAndSpace(t *testing.T) {
	got, _ := complete("lifecycle vm-a re", nil, "", nil)
	if got != "lifecycle vm-a restart " {
		t.Errorf("complete = %q, want \"lifecycle vm-a restart \"", got)
	}
}

func TestCompleteCommonPrefixOnAmbiguous(t *testing.T) {
	// host verb sub-options add/list/rm/probe; "" → no common prefix beyond ""
	got, cands := complete("host ", nil, "", nil)
	if got != "host " {
		t.Errorf("ambiguous complete should not change line, got %q", got)
	}
	if len(cands) != 4 {
		t.Errorf("expected 4 host sub candidates, got %v", cands)
	}
}

func TestCompleteFreeformNoCompletion(t *testing.T) {
	got, cands := complete("db vm-a exec sel", nil, "", nil) // exec's next slot is argFree (sql)
	if got != "db vm-a exec sel" || len(cands) != 0 {
		t.Errorf("freeform should not complete: line=%q cands=%v", got, cands)
	}
}

func TestCompleteUnknownVerbNoCandidates(t *testing.T) {
	_, cands := complete("frob ", nil, "", nil)
	if len(cands) != 0 {
		t.Errorf("unknown verb args → no candidates, got %v", cands)
	}
}

// TestSuggestAdminCatalog_EmptyToken verifies that no catalog candidates are
// returned for an empty token at the name position.
func TestSuggestAdminCatalog_EmptyToken(t *testing.T) {
	// "admin vm-a item player-x " — trailing space means next token is empty
	got := suggest("admin vm-a item player-x ", nil, "", nil)
	if got != nil {
		t.Errorf("catalog slot with empty token: expected nil, got %d candidates", len(got))
	}
}

// TestSuggestAdminCatalog_PrefixFiltered verifies that a prefix at the name
// position returns only matching catalog ids.
func TestSuggestAdminCatalog_PrefixFiltered(t *testing.T) {
	// "admin vm-a item player-x T6_Augment_Ac" should match T6_Augment_Acuracy1
	got := suggest("admin vm-a item player-x T6_Augment_Ac", nil, "", nil)
	if len(got) == 0 {
		t.Fatal("catalog suggest with prefix T6_Augment_Ac: got no candidates")
	}
	found := false
	for _, c := range got {
		if c == "T6_Augment_Acuracy1" {
			found = true
		}
	}
	if !found {
		t.Errorf("T6_Augment_Acuracy1 not in suggest results: %v", got)
	}
}

// TestCompleteAdminCatalog_PrefixComplete verifies that Tab-complete on a
// unique catalog prefix inserts the full value and a trailing space.
// "Skills.Ability.Hypersp" is a unique prefix for "Skills.Ability.Hypersprint".
func TestCompleteAdminCatalog_PrefixComplete(t *testing.T) {
	line, cands := complete("admin vm-a skill player-x Skills.Ability.Hypersp", nil, "", nil)
	if len(cands) == 0 {
		t.Fatal("complete on unique skill prefix: no candidates")
	}
	// Unique match → complete to the full value + trailing space.
	if len(cands) == 1 {
		if line != "admin vm-a skill player-x Skills.Ability.Hypersprint " {
			t.Errorf("unique catalog complete: got %q, want full value + space", line)
		}
	} else {
		// Multiple matches: line must at least have advanced beyond the input.
		if line == "admin vm-a skill player-x Skills.Ability.Hypersp" {
			t.Errorf("ambiguous catalog complete: line did not advance, got %q", line)
		}
	}
}

// TestSuggestAdminCatalog_SkillVerb verifies that the skill catalog is used
// when the admin sub-verb is "skill".
func TestSuggestAdminCatalog_SkillVerb(t *testing.T) {
	got := suggest("admin vm-a skill player-x Skills.Ability.H", nil, "", nil)
	if len(got) == 0 {
		t.Fatal("skill catalog suggest: got no candidates")
	}
	// All results should be skill module ids (no item ids).
	for _, c := range got {
		if len(c) > 0 && c[0] == 'T' {
			// item ids start with 'T' (e.g. T6_...), skill ids start with 'Skills.'
			t.Errorf("skill catalog returned non-skill id: %q", c)
		}
	}
}

// TestSuggestPlayerUsesCache verifies prefix filtering against the name cache.
func TestSuggestPlayerUsesCache(t *testing.T) {
	cache := map[string][]string{"vm-a": {"Stilgar", "Stilburn", "Muad'Dib"}}
	got := suggest("whisper vm-a Stil", []string{"vm-a"}, "vm-a", cache)
	if len(got) != 2 || got[0] != "Stilgar" {
		t.Fatalf("player suggest = %v", got)
	}
}

// TestSuggestPlayerEmptyTokenListsAll verifies that an empty token lists all
// cached names (no suppression for argPlayer, unlike argCatalog).
func TestSuggestPlayerEmptyTokenListsAll(t *testing.T) {
	cache := map[string][]string{"vm-a": {"A", "B", "C"}}
	got := suggest("whisper vm-a ", []string{"vm-a"}, "vm-a", cache)
	if len(got) != 3 {
		t.Fatalf("empty token should list all players, got %v", got)
	}
}

func TestSuggestPlayerWithImpliedHost(t *testing.T) {
	cache := map[string][]string{"vm-a": {"Stilgar", "Stilburn", "Muad'Dib"}}
	got := suggest("whisper Sti", []string{"vm-a"}, "vm-a", cache)
	if len(got) != 2 || got[0] != "Stilgar" {
		t.Fatalf("implied-host player suggest = %v", got)
	}
	all := suggest("whisper ", []string{"vm-a"}, "vm-a", cache)
	if len(all) != 3 {
		t.Fatalf("implied-host empty token = %v", all)
	}
}

func TestCompleteImpliedHostKeepsHostOutOfLine(t *testing.T) {
	cache := map[string][]string{"vm-a": {"Stilgar"}}
	line, _ := complete("whisper Sti", []string{"vm-a"}, "vm-a", cache)
	// unique match adds a trailing space; the host must NOT appear in the line
	if line != "whisper Stilgar " {
		t.Fatalf("complete must rebuild the RAW line (no host): %q", line)
	}
}

func TestSuggestNameCompletionImpliedHost(t *testing.T) {
	hosts := []string{"vm-a", "vm-b"}
	cache := map[string][]string{"vm-a": {"Malik", "Mallory", "Zed"}}
	// implied host: "player inspect Mal" → name slot, prefix "Mal".
	got := suggest("player inspect Mal", hosts, "vm-a", cache)
	if len(got) != 2 || got[0] != "Malik" || got[1] != "Mallory" {
		t.Fatalf("implied-host name completion = %v, want [Malik Mallory]", got)
	}
	// explicit host: "player vm-a inspect Mal" → same result.
	got = suggest("player vm-a inspect Mal", hosts, "vm-a", cache)
	if len(got) != 2 {
		t.Fatalf("explicit-host name completion = %v, want 2", got)
	}
}

// The H-fix: admin's SubArgs catalog/player slots resolve correctly under an
// implied host (sub-verb token lands at the normalised index 2).
func TestSuggestAdminImpliedHostSubArgs(t *testing.T) {
	hosts := []string{"vm-a"}
	cache := map[string][]string{"vm-a": {"Malik"}}
	// implied host: "admin skill Mal" → player-id slot (idx 2) → name cache.
	got := suggest("admin skill Mal", hosts, "vm-a", cache)
	if len(got) != 1 || got[0] != "Malik" {
		t.Fatalf("implied-host admin player completion = %v, want [Malik]", got)
	}
	// implied host: catalog slot for the skill verb resolves to skills (idx 3
	// (0-based) after normalisation): "admin skill player-x Skills.Ability.H".
	got = suggest("admin skill player-x Skills.Ability.H", hosts, "vm-a", cache)
	if len(got) == 0 {
		t.Fatalf("implied-host admin skill catalog should resolve, got empty")
	}
	for _, c := range got {
		if !strings.HasPrefix(c, "Skills.Ability.H") {
			t.Fatalf("catalog candidate %q does not match prefix", c)
		}
	}
}

func TestNormalizeTokensInsertsImpliedHost(t *testing.T) {
	w, _ := command.SpecFor("whisper")
	hosts := []string{"vm-a"}
	got := normalizeTokens(w, []string{"whisper"}, "vm-a", hosts)
	if len(got) != 2 || got[1] != "vm-a" {
		t.Fatalf("normalize = %v, want [whisper vm-a]", got)
	}
	// explicit host is not doubled.
	got = normalizeTokens(w, []string{"whisper", "vm-a"}, "vm-a", hosts)
	if len(got) != 2 {
		t.Fatalf("explicit host normalize = %v, want unchanged", got)
	}
	h, _ := command.SpecFor("host") // not host-first: first arg is argFixed "sub"
	got = normalizeTokens(h, []string{"host"}, "vm-a", hosts)
	if len(got) != 1 {
		t.Fatalf("non-host-first: normalize = %v, want unchanged", got)
	}
}

func TestSuggestAvatarTransferSlots(t *testing.T) {
	hosts := []string{"vm-a", "vm-b"}
	cache := map[string][]string{"vm-a": {"Malik"}}
	// implied src host: "avatar transfer " → dst-host slot → host candidates.
	got := suggest("avatar transfer ", hosts, "vm-a", cache)
	if len(got) != 2 {
		t.Fatalf("avatar transfer dst-host = %v, want 2 hosts", got)
	}
	// next slot is the player name.
	got = suggest("avatar transfer vm-b Mal", hosts, "vm-a", cache)
	if len(got) != 1 || got[0] != "Malik" {
		t.Fatalf("avatar transfer name = %v, want [Malik]", got)
	}
}

func TestUsageHintShowsFlags(t *testing.T) {
	got := usageHint("item set")
	if !strings.Contains(got, "--qty") || !strings.Contains(got, "--confirm") {
		t.Fatalf("usage hint for 'item set' should show flags, got %q", got)
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
