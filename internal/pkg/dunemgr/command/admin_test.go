package command

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// newCoreForAdmin builds a minimal Core with a temp store and a no-op SSH
// client. The SSH runner never fires in these unit tests; errors come from
// argument validation before any network call.
func newCoreForAdmin(t *testing.T) *core.Core {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "state.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &core.Core{Store: st, SSH: ssh.NewClient()}
}

// --- usage / validation ---

func TestAdminCmd_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := adminCmd(context.Background(), nil, nil, &stdout, &stderr); err == nil {
		t.Fatal("expected error for no args, got nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestAdminCmd_TooFewArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Missing player-id.
	if err := adminCmd(context.Background(), nil, []string{"vm-a", "kick"}, &stdout, &stderr); err == nil {
		t.Fatal("expected error for too few args, got nil")
	}
}

func TestAdminCmd_UnknownVerb(t *testing.T) {
	c := newCoreForAdmin(t)
	var stdout, stderr bytes.Buffer
	err := adminCmd(context.Background(), c, []string{"vm-a", "notaverb", "player-x"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown verb, got nil")
	}
	if !strings.Contains(stderr.String(), "notaverb") {
		t.Errorf("stderr does not mention verb: %q", stderr.String())
	}
}

// TestAdminCmd_DestructiveWithoutConfirm verifies that kick without --confirm
// returns an error without attempting a publish. Uses --id to skip name
// resolution so the destructive-gate is what fails, not the resolver.
func TestAdminCmd_DestructiveWithoutConfirm(t *testing.T) {
	c := newCoreForAdmin(t)
	var stdout, stderr bytes.Buffer
	err := adminCmd(context.Background(), c, []string{"vm-a", "kick", "player-x", "--id"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for kick without --confirm, got nil")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("error does not mention 'destructive': %v", err)
	}
}

// TestAdminCmd_CleanWithoutConfirm is a second destructive-gate test.
// Uses --id to skip name resolution so the destructive-gate is what fails.
func TestAdminCmd_CleanWithoutConfirm(t *testing.T) {
	c := newCoreForAdmin(t)
	var stdout, stderr bytes.Buffer
	err := adminCmd(context.Background(), c, []string{"vm-a", "clean", "player-x", "--id"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for clean without --confirm, got nil")
	}
}

// TestAdminCmd_WildcardWithoutConfirm verifies that item * without --confirm
// returns an error.
func TestAdminCmd_WildcardWithoutConfirm(t *testing.T) {
	c := newCoreForAdmin(t)
	var stdout, stderr bytes.Buffer
	err := adminCmd(context.Background(), c, []string{"vm-a", "item", "*", "SomeItem"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for wildcard without --confirm, got nil")
	}
}

// TestAdminCmd_ItemMissingItemName verifies that "item" without a name errors
// before any publish. Uses --id to skip name resolution so ItemName validation
// is what fails.
func TestAdminCmd_ItemMissingItemName(t *testing.T) {
	c := newCoreForAdmin(t)
	var stdout, stderr bytes.Buffer
	err := adminCmd(context.Background(), c, []string{"vm-a", "item", "player-x", "--id"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for item without ItemName, got nil")
	}
	if !strings.Contains(err.Error(), "ItemName") {
		t.Errorf("error does not mention ItemName: %v", err)
	}
}

// --- flag parsing ---

// TestAdminCmd_FlagParse_Water verifies that --amount maps to WaterAmount.
// Tests the pure parser directly (no SSH/publish).
func TestAdminCmd_FlagParse_Water(t *testing.T) {
	var stderr bytes.Buffer
	fields, confirm, err := parseAdminFlags("water", []string{"--amount", "500"}, &stderr)
	if err != nil {
		t.Fatalf("parseAdminFlags water: %v", err)
	}
	if fields["WaterAmount"] != "500" {
		t.Errorf("WaterAmount = %q, want \"500\"", fields["WaterAmount"])
	}
	if confirm {
		t.Error("confirm should be false")
	}
}

// TestAdminCmd_FlagParse_Item verifies positional ItemName + --qty.
func TestAdminCmd_FlagParse_Item(t *testing.T) {
	var stderr bytes.Buffer
	fields, _, err := parseAdminFlags("item", []string{"BP_Item_Resource_Spice_C", "--qty", "3"}, &stderr)
	if err != nil {
		t.Fatalf("parseAdminFlags item: %v", err)
	}
	if fields["ItemName"] != "BP_Item_Resource_Spice_C" {
		t.Errorf("ItemName = %q, want the positional value", fields["ItemName"])
	}
	if fields["Quantity"] != "3" {
		t.Errorf("Quantity = %q, want \"3\"", fields["Quantity"])
	}
}

// TestAdminSkillpointsRequiresPoints verifies that "admin skillpoints" without
// --points returns ErrUsage and steers the operator to "give skillpoints".
// c is nil to prove the usage check fires before any Core dereference.
func TestAdminSkillpointsRequiresPoints(t *testing.T) {
	var out, errb bytes.Buffer
	err := adminCmd(context.Background(), nil, []string{"vm-a", "skillpoints", "Mal"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("admin skillpoints without --points: err=%v want ErrUsage", err)
	}
	if !strings.Contains(errb.String(), "give skillpoints") {
		t.Errorf("usage should steer to give skillpoints: %q", errb.String())
	}
}

// TestAdminUnknownPlayerIsUsageError verifies that passing a player NAME (not a
// raw FLS id or wildcard) resolves via the DB; if resolution fails (no host
// reachable in tests) the error is wrapped as ErrUsage.
func TestAdminUnknownPlayerIsUsageError(t *testing.T) {
	c := newCoreForAdmin(t) // has ssh.NewClient(); discoverDB fails → ErrUsage
	var out, errb bytes.Buffer
	// "water NoSuchName" — non-destructive verb so no --confirm needed before resolution.
	err := adminCmd(context.Background(), c, []string{"h", "water", "NoSuchName"}, &out, &errb)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("unknown name should be ErrUsage, got %v", err)
	}
}

// TestAdminCmd_SpecRegistered verifies the "admin" spec is present and has the
// right shape (first arg = host, second = fixed verb set, SubArgs per verb).
func TestAdminCmd_SpecRegistered(t *testing.T) {
	spec, ok := SpecFor("admin")
	if !ok {
		t.Fatal("admin spec not registered")
	}
	if spec.Summary == "" {
		t.Error("admin spec has empty summary")
	}
	// Args holds host + verb; per-verb slots live in SubArgs.
	if len(spec.Args) != 2 {
		t.Errorf("admin spec has %d Args, want 2 (host + verb)", len(spec.Args))
	}
	if len(spec.SubArgs) == 0 {
		t.Error("admin spec SubArgs must be populated")
	}
	// arg[0] = host, arg[1] = fixed verbs
	candidates := spec.Candidates(1, nil)
	if len(candidates) == 0 {
		t.Error("admin verb completion candidates are empty")
	}
	found := false
	for _, c := range candidates {
		if c == "kick" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'kick' missing from admin candidates: %v", candidates)
	}
}
