package dbquery

import (
	"context"
	"strings"
	"testing"
)

func TestInventoryItemsParses(t *testing.T) {
	// Two rows: id|template|stack|quality
	reply := "8841|Spice|120|5\n9002|SandBoots|1|3\n"
	r, rr := newXferRunner(t, reply)

	items, err := r.InventoryItems(context.Background(), "vm-a", "DEADBEEF", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items=%d, want 2", len(items))
	}
	first := items[0]
	if first.ID != 8841 {
		t.Errorf("ID=%d, want 8841", first.ID)
	}
	if first.TemplateID != "Spice" {
		t.Errorf("TemplateID=%q, want Spice", first.TemplateID)
	}
	if first.StackSize != 120 {
		t.Errorf("StackSize=%d, want 120", first.StackSize)
	}
	if first.Quality != 5 {
		t.Errorf("Quality=%d, want 5", first.Quality)
	}

	// SQL must be pawn-scoped via inv.actor_id.
	if !strings.Contains(rr.stdinSQL, "inv.actor_id") {
		t.Errorf("SQL missing inv.actor_id (not pawn-scoped): %q", rr.stdinSQL)
	}
	// fls must be bound via :'fls', not interpolated.
	if strings.Contains(rr.stdinSQL, "DEADBEEF") {
		t.Errorf("fls value leaked into SQL body: %q", rr.stdinSQL)
	}
	if !strings.Contains(rr.stdinSQL, ":'fls'") {
		t.Errorf("SQL missing :'fls' reference: %q", rr.stdinSQL)
	}
	// inventory_type must be bound via :invtype.
	if !strings.Contains(rr.stdinSQL, ":invtype") {
		t.Errorf("SQL missing :invtype reference: %q", rr.stdinSQL)
	}
	// fls var must be passed via -v.
	if !hasVar(rr.args, "fls=DEADBEEF") {
		t.Errorf("fls not bound as -v: %v", rr.args)
	}
	// invtype var must be passed via -v.
	if !hasVar(rr.args, "invtype=3") {
		t.Errorf("invtype not bound as -v: %v", rr.args)
	}
}

func TestInventoryItemsEmpty(t *testing.T) {
	r, _ := newXferRunner(t, "")
	items, err := r.InventoryItems(context.Background(), "vm-a", "DEADBEEF", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("items=%d, want 0", len(items))
	}
}

func TestSetItemStack(t *testing.T) {
	r, rr := newXferRunner(t, "8841\n")
	id, err := r.SetItemStack(context.Background(), "vm-a", 8841, 50)
	if err != nil {
		t.Fatal(err)
	}
	if id != 8841 {
		t.Errorf("id=%d, want 8841", id)
	}
	// SQL must target the right table and column.
	if !strings.Contains(rr.stdinSQL, "UPDATE dune.items") {
		t.Errorf("SQL missing UPDATE dune.items: %q", rr.stdinSQL)
	}
	if !strings.Contains(rr.stdinSQL, "stack_size = :stack::bigint") {
		t.Errorf("SQL missing stack_size = :stack::bigint: %q", rr.stdinSQL)
	}
	// Pawn scope must join through player_pawn_id = inv.actor_id.
	if !strings.Contains(rr.stdinSQL, "player_pawn_id = inv.actor_id") {
		t.Errorf("SQL missing pawn scope (player_pawn_id = inv.actor_id): %q", rr.stdinSQL)
	}
	// item ID must be bound as :item, not interpolated.
	if strings.Contains(rr.stdinSQL, "8841") {
		t.Errorf("item id leaked into SQL body: %q", rr.stdinSQL)
	}
	if !strings.Contains(rr.stdinSQL, ":item::bigint") {
		t.Errorf("SQL missing :item::bigint: %q", rr.stdinSQL)
	}
	if !strings.Contains(rr.stdinSQL, "RETURNING id") {
		t.Errorf("SQL missing RETURNING id: %q", rr.stdinSQL)
	}
	// ON_ERROR_STOP must be prepended.
	if !strings.HasPrefix(rr.stdinSQL, `\set ON_ERROR_STOP on`) {
		t.Errorf("stdinSQL missing ON_ERROR_STOP prefix: %q", rr.stdinSQL)
	}
	if !hasVar(rr.args, "item=8841") {
		t.Errorf("item not bound as -v: %v", rr.args)
	}
	if !hasVar(rr.args, "stack=50") {
		t.Errorf("stack not bound as -v: %v", rr.args)
	}
}

func TestSetItemQuality(t *testing.T) {
	r, rr := newXferRunner(t, "8841\n")
	id, err := r.SetItemQuality(context.Background(), "vm-a", 8841, 3)
	if err != nil {
		t.Fatal(err)
	}
	if id != 8841 {
		t.Errorf("id=%d, want 8841", id)
	}
	if !strings.Contains(rr.stdinSQL, "quality_level = :quality::bigint") {
		t.Errorf("SQL missing quality_level = :quality::bigint: %q", rr.stdinSQL)
	}
	if !hasVar(rr.args, "quality=3") {
		t.Errorf("quality not bound as -v: %v", rr.args)
	}
}

func TestDeleteItem(t *testing.T) {
	r, rr := newXferRunner(t, "8841\n")
	id, err := r.DeleteItem(context.Background(), "vm-a", 8841)
	if err != nil {
		t.Fatal(err)
	}
	if id != 8841 {
		t.Errorf("id=%d, want 8841", id)
	}
	if !strings.Contains(rr.stdinSQL, "DELETE FROM dune.items") {
		t.Errorf("SQL missing DELETE FROM dune.items: %q", rr.stdinSQL)
	}
	if !strings.Contains(rr.stdinSQL, "player_pawn_id = inv.actor_id") {
		t.Errorf("SQL missing pawn scope: %q", rr.stdinSQL)
	}
}

func TestItemOwnerFLS(t *testing.T) {
	r, rr := newXferRunner(t, "DEADBEEF\n")
	fls, err := r.ItemOwnerFLS(context.Background(), "vm-a", 8841)
	if err != nil {
		t.Fatal(err)
	}
	if fls != "DEADBEEF" {
		t.Errorf("fls=%q, want DEADBEEF", fls)
	}
	if !hasVar(rr.args, "item=8841") {
		t.Errorf("item not bound as -v: %v", rr.args)
	}
}

func TestSetItemStackNoRow(t *testing.T) {
	r, _ := newXferRunner(t, "")
	id, err := r.SetItemStack(context.Background(), "vm-a", 8841, 50)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Errorf("id=%d, want 0 for no-row", id)
	}
}

func TestSetItemStackIgnoresCommandTag(t *testing.T) {
	r, _ := newXferRunner(t, "1496696\nUPDATE 1\n")
	n, err := r.SetItemStack(context.Background(), "vm-a", 1496696, 5)
	if err != nil {
		t.Fatalf("must parse the id line, not choke on the command tag: %v", err)
	}
	if n != 1496696 {
		t.Fatalf("n=%d want 1496696", n)
	}
}
