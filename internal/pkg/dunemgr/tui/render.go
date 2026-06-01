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
)

// renderHostList renders one row per host with a cursor on the selected index.
func renderHostList(hosts []string, st map[string]hostStatus, selected int) string {
	if len(hosts) == 0 {
		return "(no hosts — add one with ':host add <name>')\n"
	}
	var b strings.Builder
	for i, h := range hosts {
		cursor := " "
		if i == selected {
			cursor = ">"
		}
		s := st[h]
		state := s.bgState
		if state == "" {
			state = "…"
		}
		row := fmt.Sprintf("%s %-14s %-9s %d/%d", cursor, h, state, s.ready, s.total)
		if i == selected {
			row = styleSelected.Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

// renderDetail renders the detail block for one host.
func renderDetail(host string, s hostStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "host:    %s\n", host)
	fmt.Fprintf(&b, "state:   %s\n", valOr(s.bgState, "unknown"))
	fmt.Fprintf(&b, "pods:    %d/%d\n", s.ready, s.total)
	reach := "○ probe error"
	if s.reachable {
		reach = "● reachable"
	}
	fmt.Fprintf(&b, "health:  %s\n", reach)
	if s.err != "" {
		fmt.Fprint(&b, styleErr.Render("error:   "+s.err))
		b.WriteString("\n")
	}
	if s.lastAction != "" {
		fmt.Fprintf(&b, "last:    %s\n", s.lastAction)
	}
	return b.String()
}

// renderEvents renders up to max recent event lines (newest last).
func renderEvents(events []string, max int) string {
	if len(events) == 0 {
		return "(no events yet)\n"
	}
	start := 0
	if len(events) > max {
		start = len(events) - max
	}
	return strings.Join(events[start:], "\n") + "\n"
}

// renderOutput renders the command result pane with a titled separator.
func renderOutput(out string) string {
	return "── result ──\n" + out + "────────────\n"
}

func valOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
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
