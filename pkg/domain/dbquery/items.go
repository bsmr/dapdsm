package dbquery

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// itemPawnScope restricts an item to a row owned by some player pawn's inventory.
const itemPawnScope = `id = :item::bigint AND inventory_id IN (
  SELECT inv.id FROM dune.inventories inv
  JOIN dune.player_state ps ON ps.player_pawn_id = inv.actor_id)`

func (r *Runner) execItemMutation(ctx context.Context, host, sql string, itemID int64, extra map[string]string) (int64, error) {
	vars := map[string]string{"item": strconv.FormatInt(itemID, 10)}
	for k, v := range extra {
		vars[k] = v
	}
	res, err := r.execWithVars(ctx, host, onErrorStop+sql, vars)
	if err != nil {
		return 0, err
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return 0, nil
	}
	// A psql mutation prints the RETURNING id on the first line, then the
	// command tag (e.g. "UPDATE 1") on the next — take only the id line.
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		out = strings.TrimSpace(out[:i])
	}
	return strconv.ParseInt(out, 10, 64)
}

// SetItemStack sets stack_size for a player-owned item. Returns the affected id
// (0 if no matching player item). Caller gates offline/confirm.
func (r *Runner) SetItemStack(ctx context.Context, host string, itemID, stack int64) (int64, error) {
	sql := `UPDATE dune.items SET stack_size = :stack::bigint WHERE ` + itemPawnScope + ` RETURNING id;`
	return r.execItemMutation(ctx, host, sql, itemID, map[string]string{"stack": strconv.FormatInt(stack, 10)})
}

// SetItemQuality sets quality_level for a player-owned item.
func (r *Runner) SetItemQuality(ctx context.Context, host string, itemID, quality int64) (int64, error) {
	sql := `UPDATE dune.items SET quality_level = :quality::bigint WHERE ` + itemPawnScope + ` RETURNING id;`
	return r.execItemMutation(ctx, host, sql, itemID, map[string]string{"quality": strconv.FormatInt(quality, 10)})
}

// DeleteItem removes a player-owned item.
func (r *Runner) DeleteItem(ctx context.Context, host string, itemID int64) (int64, error) {
	sql := `DELETE FROM dune.items WHERE ` + itemPawnScope + ` RETURNING id;`
	return r.execItemMutation(ctx, host, sql, itemID, nil)
}

// ItemOwnerFLS resolves the FLS id of the player owning itemID, or "" if the
// item is not owned by any player pawn. Read-only.
func (r *Runner) ItemOwnerFLS(ctx context.Context, host string, itemID int64) (string, error) {
	sql := `SELECT a."user"::text
FROM dune.items i
JOIN dune.inventories inv ON inv.id = i.inventory_id
JOIN dune.player_state ps ON ps.player_pawn_id = inv.actor_id
JOIN dune.accounts a ON a.id = ps.account_id
WHERE i.id = :item::bigint LIMIT 1;`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{"item": strconv.FormatInt(itemID, 10)})
	if err != nil {
		return "", fmt.Errorf("item owner: %w", err)
	}
	return strings.TrimSpace(res.Stdout), nil
}

// ItemRow is one item stack in a player's inventory (read-only listing).
type ItemRow struct {
	ID         int64
	TemplateID string
	StackSize  int64
	Quality    int64
}

// InventoryItems lists the item stacks in the player's inventory of the given
// inventory_type (the pawn resolved from fls). Read-only; no audit.
func (r *Runner) InventoryItems(ctx context.Context, host, fls string, inventoryType int) ([]ItemRow, error) {
	sql := `SELECT i.id, COALESCE(i.template_id,''), i.stack_size, i.quality_level
FROM dune.items i
JOIN dune.inventories inv ON inv.id = i.inventory_id
WHERE inv.actor_id = ` + pawnSubquery + `
  AND inv.inventory_type = :invtype::int
ORDER BY i.position_index;`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{
		"fls": fls, "invtype": strconv.Itoa(inventoryType),
	})
	if err != nil {
		return nil, fmt.Errorf("inventory items: %w", err)
	}
	var out []ItemRow
	for _, l := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		p := strings.Split(l, "|")
		if len(p) != 4 {
			continue
		}
		id, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("inventory items: parse id %q: %w", p[0], err)
		}
		ss, _ := strconv.ParseInt(p[2], 10, 64)
		q, _ := strconv.ParseInt(p[3], 10, 64)
		out = append(out, ItemRow{ID: id, TemplateID: p[1], StackSize: ss, Quality: q})
	}
	return out, nil
}
