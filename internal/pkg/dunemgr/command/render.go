package command

import (
	"fmt"
	"strings"
	"text/tabwriter"

	admincatalog "go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin/catalog"
	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
)

// inventoryTypeNames maps Funcom's raw dune.inventories.inventory_type integer
// to a human label. Live-confirmed on the running server (vm-dune-01):
// 0=Backpack, 1=Equipment(worn armor), 14=Emotes, 15=Hotbar(tools),
// 27=Emotes(2), 29=Contracts. 30 is in dune-admin's gear set but absent live.
// Unknown ids fall back to "type N". dune.inventories has no name column, so
// this is the only source of a readable name.
var inventoryTypeNames = map[int]string{
	0: "Backpack", 1: "Equipment", 14: "Emotes", 15: "Hotbar", 27: "Emotes (2)", 29: "Contracts",
}

// InventoryTypeName returns a human label for an inventory_type, or "type N".
func InventoryTypeName(t int) string {
	if n, ok := inventoryTypeNames[t]; ok {
		return n
	}
	return fmt.Sprintf("type %d", t)
}

// FormatInspect renders a player detail (header + inventory + progression +
// vitals + spice) as a string. The RawComponents block is excluded; callers
// that need it should append it separately. Returns a "no player" line when
// d.Found is false.
func FormatInspect(d *dbquery.PlayerDetail) string {
	var b strings.Builder
	if !d.Found {
		fmt.Fprintf(&b, "no player with fls %s\n", d.FLSID)
		return b.String()
	}
	fmt.Fprintf(&b, "player %s (%s)\n", d.CharacterName, d.FLSID)
	fmt.Fprintf(&b, "  status=%s  last-seen=%s  partition=%s\n", d.OnlineStatus, d.LastSeen, valueOrDash(d.Partition))
	fmt.Fprintf(&b, "  inventory: %d items in %d inventories, %d total stacks\n",
		d.ItemCount, len(d.Inventories), d.StackTotal)
	for i, inv := range d.Inventories {
		fmt.Fprintf(&b, "    [%d] %s: %d items\n", i+1, InventoryTypeName(inv.InventoryType), inv.ItemCount)
	}
	if len(d.TopItems) > 0 {
		fmt.Fprintln(&b, "  top items by quality:")
		tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "    TEMPLATE\tSTACK\tQUALITY")
		for _, it := range d.TopItems {
			fmt.Fprintf(tw, "    %s\t%d\t%d\n", it.TemplateID, it.StackSize, it.Quality)
		}
		_ = tw.Flush()
	}
	if d.Progression != nil {
		p := d.Progression
		fmt.Fprintf(&b, "  progression: skill points %d total / %d unspent  xp=%d  keystone=%d  respecs=%d\n",
			p.TotalSkillPoints, p.UnspentSkillPoints, p.TotalXPEarned, p.KeystoneBonusSkillPoints, p.RespecCostCounter)
		fmt.Fprintf(&b, "               perks=%d  abilities=%d\n", p.ActivePerks, p.ActiveAbilities)
	}
	if d.Vitals != nil {
		fmt.Fprintf(&b, "  vitals:      health %.0f  (down-but-not-out %.0f/%.0f)\n",
			d.Vitals.CurrentHealth, d.Vitals.DownCurrent, d.Vitals.DownMax)
	}
	if d.Spice != nil {
		fmt.Fprintf(&b, "  spice:       %s  vision=%s\n", d.Spice.SystemStatus, d.Spice.SpiceVision)
	}
	return b.String()
}

// FormatInventoryItems renders the item stacks of one inventory type, numbered,
// each with its dune.items id (the handle for `item set`/`item delete`).
func FormatInventoryItems(inventoryType int, rows []dbquery.ItemRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d stacks\n", InventoryTypeName(inventoryType), len(rows))
	if len(rows) == 0 {
		return b.String()
	}
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  #\tID\tTEMPLATE\tSTACK\tQUALITY")
	for i, it := range rows {
		fmt.Fprintf(tw, "  [%d]\tid=%d\t%s\t%d\t%d\n", i+1, it.ID, admincatalog.DisplayName(it.TemplateID), it.StackSize, it.Quality)
	}
	_ = tw.Flush()
	return b.String()
}

// valueOrDash returns s unchanged, or "-" if s is empty.
func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
