package tui

import (
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
)

// injectHost rewrites a command-bar line so the selected host is supplied for
// verbs whose spec starts with an argHost slot, unless the operator already
// typed a known host alias in that position. Returns line unchanged when there
// is no selection, the verb has no leading host slot, or an explicit alias is
// present.
func injectHost(line, selected string, hosts []string) string {
	if selected == "" {
		return line
	}
	f := strings.Fields(line)
	if len(f) == 0 {
		return line
	}
	spec, ok := command.SpecFor(f[0])
	if !ok || !spec.FirstArgIsHost() {
		return line
	}
	if len(f) >= 2 && isKnownHost(f[1], hosts) {
		return line // explicit host wins
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, f[0]))
	if rest == "" {
		return f[0] + " " + selected
	}
	return f[0] + " " + selected + " " + rest
}

func isKnownHost(tok string, hosts []string) bool {
	for _, h := range hosts {
		if h == tok {
			return true
		}
	}
	return false
}
