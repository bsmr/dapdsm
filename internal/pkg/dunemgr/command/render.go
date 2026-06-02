package command

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
)

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
	for _, inv := range d.Inventories {
		fmt.Fprintf(&b, "    inv-type %d: %d items\n", inv.InventoryType, inv.ItemCount)
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

// valueOrDash returns s unchanged, or "-" if s is empty.
func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
