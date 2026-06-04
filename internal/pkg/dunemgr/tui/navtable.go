// Package tui — navtable lays nav-drill list data into aligned, optionally
// sort-marked columns shared between the pinned header and the data rows.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type cellAlign int

const (
	cellLeft cellAlign = iota
	cellRight
)

// navColumn describes one column of a nav-list table.
type navColumn struct {
	label    string
	align    cellAlign
	sortable bool
}

// navColGap separates adjacent columns.
const navColGap = "  "

// sortMarker returns the 1-rune header indicator for column colIdx: '^' (asc) /
// 'v' (desc) on the active sortable column, a reserved space on other sortable
// columns (so width stays constant), and "" on non-sortable columns.
func sortMarker(colIdx, sortCol int, sortable, desc bool) string {
	if !sortable {
		return ""
	}
	if colIdx == sortCol {
		if desc {
			return "v"
		}
		return "^"
	}
	return " "
}

// formatTable lays cells (rows × cols) into aligned text and returns the header
// line plus one formatted string per row, sharing per-column widths. ASCII sort
// markers only.
func formatTable(cols []navColumn, cells [][]string, sortCol int, desc bool) (string, []string) {
	n := len(cols)
	widths := make([]int, n)
	labels := make([]string, n)
	for j, c := range cols {
		labels[j] = c.label + sortMarker(j, sortCol, c.sortable, desc)
		widths[j] = lipgloss.Width(labels[j])
	}
	for _, row := range cells {
		for j := 0; j < n && j < len(row); j++ {
			if w := lipgloss.Width(row[j]); w > widths[j] {
				widths[j] = w
			}
		}
	}
	pad := func(s string, w int, a cellAlign) string {
		if a == cellRight {
			return fmt.Sprintf("%*s", w, s)
		}
		return fmt.Sprintf("%-*s", w, s)
	}
	line := func(get func(j int) string) string {
		parts := make([]string, n)
		for j, c := range cols {
			parts[j] = pad(get(j), widths[j], c.align)
		}
		return strings.TrimRight(strings.Join(parts, navColGap), " ")
	}
	header := line(func(j int) string { return labels[j] })
	rows := make([]string, len(cells))
	for i := range cells {
		row := cells[i]
		rows[i] = line(func(j int) string {
			if j < len(row) {
				return row[j]
			}
			return ""
		})
	}
	return header, rows
}
