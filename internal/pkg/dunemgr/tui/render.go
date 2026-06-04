package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
)

var (
	styleSelected = lipgloss.NewStyle().Bold(true)
	styleErr      = lipgloss.NewStyle().Faint(true)
	styleBox      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	styleAccent   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

// renderHeader renders all hosts as inline badges across the full width; the
// active host (selected) is marked with ▸. Host status appears ONLY here.
func renderHeader(hosts []string, st map[string]hostStatus, selected int) string {
	if len(hosts) == 0 {
		return "(no hosts — add one with ':host add <name>')"
	}
	var parts []string
	for i, h := range hosts {
		s := st[h]
		state := s.bgState
		if state == "" {
			state = "…"
		}
		badge := "○"
		if s.reachable {
			badge = "●"
		}
		mark := "  "
		if i == selected {
			mark = "▸ "
		}
		cell := fmt.Sprintf("%s%s %s %s %d/%d", mark, badge, h, state, s.ready, s.total)
		if i == selected {
			cell = styleSelected.Render(cell)
		}
		parts = append(parts, cell)
	}
	return strings.Join(parts, "   ")
}

// renderOutputScrolled renders a windowed slice of out starting at scroll,
// showing at most window lines.
func renderOutputScrolled(out string, scroll, window int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if scroll > len(lines) {
		scroll = len(lines)
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + window
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[scroll:end], "\n")
}

// helperMaxRows caps the command-mode helper line (suggestions / usage grammar)
// at this many rows before a "(+N more)" suffix is appended.
const helperMaxRows = 4

// renderSuggestions lays the completion candidates out as a width-aware grid of
// up to maxRows rows (cells sized to the widest candidate). Only when the
// candidates exceed that grid does the last cell become a "(+N more)" marker —
// so a small set (e.g. the verb list) is shown in full.
func renderSuggestions(cands []string, width, maxRows int) string {
	if len(cands) == 0 {
		return ""
	}
	if width < 1 {
		width = 1
	}
	if maxRows < 1 {
		maxRows = 1
	}
	cell := 0
	for _, c := range cands {
		if w := lipgloss.Width(c); w > cell {
			cell = w
		}
	}
	cell += 2 // inter-column gap
	perRow := width / cell
	if perRow < 1 {
		perRow = 1
	}
	capacity := perRow * maxRows
	overflow := 0
	if len(cands) > capacity {
		overflow = len(cands) - (capacity - 1) // reserve one cell for the marker
		cands = cands[:capacity-1]
	}
	var b strings.Builder
	for i, c := range cands {
		if i > 0 && i%perRow == 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%-*s", cell, c)
	}
	if overflow > 0 {
		fmt.Fprintf(&b, "%-*s", cell, fmt.Sprintf("(+%d more)", overflow))
	}
	return b.String()
}

// wrapBreak word-wraps s into lines no wider than width, breaking at spaces or
// '|' (the break character stays on the left). A token longer than width is
// hard-broken. ASCII grammar strings only.
func wrapBreak(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for lipgloss.Width(s) > width {
		cut := -1
		for i := 0; i < width && i < len(s); i++ {
			if s[i] == ' ' || s[i] == '|' {
				cut = i
			}
		}
		if cut <= 0 {
			cut = width - 1
		}
		lines = append(lines, strings.TrimRight(s[:cut+1], " "))
		s = strings.TrimLeft(s[cut+1:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

// wrapHelper wraps a (possibly multi-line) usage grammar to width, capping at
// maxRows rows; a "(+N more)" marker replaces the tail when it overflows.
func wrapHelper(s string, width, maxRows int) string {
	if s == "" {
		return ""
	}
	if maxRows < 1 {
		maxRows = 1
	}
	var all []string
	for _, ln := range strings.Split(s, "\n") {
		all = append(all, wrapBreak(ln, width)...)
	}
	if len(all) > maxRows {
		over := len(all) - (maxRows - 1)
		all = all[:maxRows-1]
		all = append(all, fmt.Sprintf("(+%d more)", over))
	}
	return strings.Join(all, "\n")
}

// helpText returns the compact key/mode reference for the ? overlay.
func helpText() string {
	return strings.Join([]string{
		"dunemgr TUI — keys",
		"",
		"Two modes:",
		"  nav mode (default): drill host → player → inventory → item",
		"  command mode: press : to type a verb (whisper, give, admin, …); Esc returns",
		"",
		"Nav keys:",
		"  ↑/↓ or k/j   move the cursor",
		"  → / Enter / l   drill into the selection",
		"  ← / Esc / h   go back one level",
		"  PgUp/PgDn   page      g/G   first/last",
		"",
		"Item level (offline player):",
		"  +/-  stack ±1    e  set qty    Q  set quality    d  delete (y/n)",
		"  (online owner refused; use the CLI: item … --force)",
		"",
		"  :  command mode    ?  this help    q  quit",
	}, "\n")
}

// framedBox renders body inside the rounded border with title embedded in the
// top edge: ╭─ title ───╮. height>0 pads the box to that many total rows.
func framedBox(title, body string, width, height int) string {
	st := styleBox.Width(width)
	if height > 0 {
		// Height sets the inner content height; subtract the 2 border rows so
		// the rendered box is exactly `height` rows tall.
		ih := height - 2
		if ih < 1 {
			ih = 1
		}
		st = st.Height(ih)
	}
	rendered := st.Render(body)
	if title == "" {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}
	b := lipgloss.RoundedBorder()
	inner := lipgloss.Width(lines[0]) - lipgloss.Width(b.TopLeft) - lipgloss.Width(b.TopRight)
	if inner < 1 {
		return rendered
	}
	label := "─ " + title + " "
	if lipgloss.Width(label) > inner {
		keep := inner - 4 // room for "─ " + "… "
		if keep < 0 {
			keep = 0
		}
		label = "─ " + truncateRunes(title, keep) + "… "
		if lipgloss.Width(label) > inner {
			label = "" // give up on the title; plain edge
		}
	}
	fill := inner - lipgloss.Width(label)
	if fill < 0 {
		fill = 0
	}
	lines[0] = b.TopLeft + label + strings.Repeat(b.Top, fill) + b.TopRight
	return strings.Join(lines, "\n")
}

// truncateRunes returns the first n runes of s.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if n < 0 {
		n = 0
	}
	if n > len(r) {
		n = len(r)
	}
	return string(r[:n])
}

// renderList renders rows with the selected row marked ▸ (and bold), scrolling
// the viewport so the selected row stays visible within height lines.
func renderList(rows []string, selected, height int) string {
	if len(rows) == 0 {
		return "(empty)"
	}
	start, end := visibleWindow(len(rows), selected, height)
	var b strings.Builder
	for i := start; i < end; i++ {
		if i == selected {
			b.WriteString("▸ " + styleSelected.Render(rows[i]) + "\n")
		} else {
			b.WriteString("  " + rows[i] + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// renderHelp renders the verb list (or one verb's detail) for the :help built-in.
func renderHelp(args []string) string {
	if len(args) > 0 {
		if s, ok := command.SpecFor(args[0]); ok {
			return s.Usage() + "\n  " + s.Summary + "\n"
		}
		return "unknown verb: " + args[0] + "\n"
	}
	var b strings.Builder
	b.WriteString("commands (Tab to complete):\n")
	for _, s := range command.Specs() {
		fmt.Fprintf(&b, "  %-44s %s\n", s.Usage(), s.Summary)
	}
	b.WriteString("  help [verb]                                  Show this help\n")
	return b.String()
}
