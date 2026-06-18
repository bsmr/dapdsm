package dbquery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PlayerDetail is the result of PlayerInspect: a header + inventory aggregate
// for one player. Structures/vehicles/landclaims/faction are intentionally not
// included — not reliably player-attributable on the live dune schema.
type PlayerDetail struct {
	FLSID         string
	Found         bool
	CharacterName string
	OnlineStatus  string
	LastSeen      string
	Partition     string
	ItemCount     int64
	StackTotal    int64
	Inventories   []InvBreakdown
	TopItems      []InvItem
	Progression   *Progression // nil if FLevelComponent absent
	Vitals        *Vitals      // nil if FHealthComponent absent
	Spice         *SpiceState  // nil if FSpiceAddictionComponent absent
	RawComponents string       // pretty components JSON when raw=true
}

// InvBreakdown is per-inventory-type item count for the player's pawn.
type InvBreakdown struct {
	InventoryType int
	ItemCount     int64
}

// InvItem is one row in the top-N-by-quality item list.
type InvItem struct {
	TemplateID string
	StackSize  int64
	Quality    int64
}

// pawnSubquery resolves the player's pawn actor id from :'fls'. Inventory hangs
// off the pawn actor (verified live).
const pawnSubquery = `(SELECT ps.player_pawn_id FROM dune.player_state ps ` +
	`JOIN dune.accounts a ON a.id = ps.account_id WHERE a."user"::text = :'fls' LIMIT 1)`

// InventoryBreakdown returns the per-inventory-type item counts for the player's
// pawn in a single query (cheaper than PlayerInspect when only the drill list is
// needed). Read-only; no audit.
func (r *Runner) InventoryBreakdown(ctx context.Context, host, fls string) ([]InvBreakdown, error) {
	sql := `SELECT inv.inventory_type, count(*)
FROM dune.items i JOIN dune.inventories inv ON inv.id = i.inventory_id
WHERE inv.actor_id = ` + pawnSubquery + `
GROUP BY inv.inventory_type ORDER BY inv.inventory_type;`
	res, err := r.execWithVars(ctx, host, sql, map[string]string{"fls": fls})
	if err != nil {
		return nil, fmt.Errorf("inventory breakdown: %w", err)
	}
	var out []InvBreakdown
	for _, l := range strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		p := strings.Split(l, "|")
		if len(p) != 2 {
			continue
		}
		it, _ := strconv.Atoi(p[0])
		c, _ := strconv.ParseInt(p[1], 10, 64)
		out = append(out, InvBreakdown{InventoryType: it, ItemCount: c})
	}
	return out, nil
}

// PlayerInspect returns a header + inventory aggregate for fls. topN bounds the
// top-by-quality item list (≤0 defaults to 10; capped at 50). When raw is true,
// the character components JSON is pretty-printed into RawComponents. Read-only; no audit.
func (r *Runner) PlayerInspect(ctx context.Context, host, fls string, topN int, raw bool) (*PlayerDetail, error) {
	if topN <= 0 {
		topN = 10
	}
	if topN > 50 {
		topN = 50
	}
	d := &PlayerDetail{FLSID: fls}
	vars := map[string]string{"fls": fls}

	headerSQL := q("inspect_header")
	hres, err := r.execWithVars(ctx, host, headerSQL, vars)
	if err != nil {
		return nil, fmt.Errorf("player inspect header: %w", err)
	}
	line := strings.TrimRight(hres.Stdout, "\n")
	if strings.TrimSpace(line) == "" {
		return d, nil
	}
	hp := strings.Split(line, "|")
	if len(hp) >= 4 {
		d.Found = true
		d.CharacterName, d.OnlineStatus, d.LastSeen, d.Partition = hp[0], hp[1], hp[2], hp[3]
	}

	totalsSQL := `SELECT count(*), COALESCE(sum(i.stack_size),0)
FROM dune.items i JOIN dune.inventories inv ON inv.id = i.inventory_id
WHERE inv.actor_id = ` + pawnSubquery + `;`
	tres, err := r.execWithVars(ctx, host, totalsSQL, vars)
	if err != nil {
		return nil, fmt.Errorf("player inspect totals: %w", err)
	}
	if tp := strings.Split(strings.TrimSpace(tres.Stdout), "|"); len(tp) == 2 {
		d.ItemCount, _ = strconv.ParseInt(tp[0], 10, 64)
		d.StackTotal, _ = strconv.ParseInt(tp[1], 10, 64)
	}

	breakdownSQL := `SELECT inv.inventory_type, count(*)
FROM dune.items i JOIN dune.inventories inv ON inv.id = i.inventory_id
WHERE inv.actor_id = ` + pawnSubquery + `
GROUP BY inv.inventory_type ORDER BY inv.inventory_type;`
	bres, err := r.execWithVars(ctx, host, breakdownSQL, vars)
	if err != nil {
		return nil, fmt.Errorf("player inspect breakdown: %w", err)
	}
	for _, l := range strings.Split(strings.TrimRight(bres.Stdout, "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		p := strings.Split(l, "|")
		if len(p) != 2 {
			continue
		}
		it, _ := strconv.Atoi(p[0])
		c, _ := strconv.ParseInt(p[1], 10, 64)
		d.Inventories = append(d.Inventories, InvBreakdown{InventoryType: it, ItemCount: c})
	}

	topSQL := `SELECT COALESCE(i.template_id,''), i.stack_size, i.quality_level
FROM dune.items i JOIN dune.inventories inv ON inv.id = i.inventory_id
WHERE inv.actor_id = ` + pawnSubquery + `
ORDER BY i.quality_level DESC, i.stack_size DESC LIMIT :lim;`
	ires, err := r.execWithVars(ctx, host, topSQL, map[string]string{"fls": fls, "lim": strconv.Itoa(topN)})
	if err != nil {
		return nil, fmt.Errorf("player inspect items: %w", err)
	}
	for _, l := range strings.Split(strings.TrimRight(ires.Stdout, "\n"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		p := strings.Split(l, "|")
		if len(p) != 3 {
			continue
		}
		ss, _ := strconv.ParseInt(p[1], 10, 64)
		q, _ := strconv.ParseInt(p[2], 10, 64)
		d.TopItems = append(d.TopItems, InvItem{TemplateID: p[0], StackSize: ss, Quality: q})
	}

	// execWithVars #5: character components (one jsonb line) for progression/vitals/spice.
	componentsSQL := q("inspect_components")
	cres, err := r.execWithVars(ctx, host, componentsSQL, vars)
	if err != nil {
		return nil, fmt.Errorf("player inspect components: %w", err)
	}
	comp := strings.TrimSpace(cres.Stdout)
	if comp != "" {
		if p, ok := parseProgression([]byte(comp)); ok {
			d.Progression = &p
		}
		if v, ok := parseVitals([]byte(comp)); ok {
			d.Vitals = &v
		}
		if s, ok := parseSpice([]byte(comp)); ok {
			d.Spice = &s
		}
		if raw {
			var buf bytes.Buffer
			if json.Indent(&buf, []byte(comp), "", "  ") == nil {
				d.RawComponents = buf.String()
			} else {
				d.RawComponents = comp
			}
		}
	}
	return d, nil
}
