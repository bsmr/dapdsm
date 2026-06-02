package tui

import (
	"sort"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
)

// helpVerb is the TUI built-in verb (handled in the model, not the dispatcher).
const helpVerb = "help"

// suggest returns the completion candidates for the current (last) token of line,
// filtered by that token as a prefix. A trailing space means the current token is
// an empty token at the next position. hosts supplies argHost candidates.
// selHost is the currently-selected host; cache is the live player-name pool
// keyed by host name.
func suggest(line string, hosts []string, selHost string, cache map[string][]string) []string {
	tokens, cur := splitCurrent(line)
	pos := len(tokens) // index of the current token
	var pool []string

	if pos == 0 {
		// verb position: dispatcher verbs + the TUI built-in "help"
		for _, s := range command.Specs() {
			pool = append(pool, s.Verb)
		}
		pool = append(pool, helpVerb)
		sort.Strings(pool)
	} else if spec, ok := command.SpecFor(tokens[0]); ok {
		// argument position: the command package owns the slot logic and returns
		// the candidate strings (nil for freeform / out-of-range).
		// Pass the already-typed tokens so argCatalog slots can select the
		// right catalog based on the admin sub-verb.
		argPos := pos - 1
		// Suppress catalog suggestions on empty token: the catalog can have
		// thousands of entries, which would flood the suggestion line.
		if spec.IsCatalogPos(argPos) && cur == "" {
			return nil
		}
		// Player slots are served from the live cache, not the static candidates.
		if spec.IsPlayerPos(argPos) {
			return suggestPlayers(line, selHost, cache)
		}
		pool = spec.Candidates(argPos, hosts, tokens...)
	}

	var out []string
	for _, c := range pool {
		if strings.HasPrefix(c, cur) {
			out = append(out, c)
		}
	}
	return out
}

// suggestPlayers returns cached player names for selHost that prefix-match the
// current token of line (empty token → all; the renderer caps display).
func suggestPlayers(line, selHost string, cache map[string][]string) []string {
	_, cur := splitCurrent(line)
	var out []string
	for _, n := range cache[selHost] {
		if cur == "" || strings.HasPrefix(strings.ToLower(n), strings.ToLower(cur)) {
			out = append(out, n)
		}
	}
	return out
}

// complete returns line with the current token completed to the longest common
// prefix of the candidates (a unique match is inserted in full + a trailing
// space), plus the candidate list. No candidates → line unchanged.
func complete(line string, hosts []string, selHost string, cache map[string][]string) (string, []string) {
	cands := suggest(line, hosts, selHost, cache)
	if len(cands) == 0 {
		return line, cands
	}
	tokens, cur := splitCurrent(line)
	lcp := longestCommonPrefix(cands)
	if len(lcp) <= len(cur) {
		return line, cands // nothing more to add
	}
	replacement := lcp
	trailing := ""
	if len(cands) == 1 {
		replacement = cands[0]
		trailing = " "
	}
	// rebuild the line with the current token replaced
	prefixTokens := tokens
	rebuilt := strings.Join(prefixTokens, " ")
	if rebuilt != "" {
		rebuilt += " "
	}
	return rebuilt + replacement + trailing, cands
}

// splitCurrent returns the completed tokens (everything before the current token)
// and the current (in-progress) token. A trailing space means the current token
// is "" at the next position.
func splitCurrent(line string) (tokens []string, current string) {
	if strings.HasSuffix(line, " ") {
		return strings.Fields(line), ""
	}
	f := strings.Fields(line)
	if len(f) == 0 {
		return nil, ""
	}
	return f[:len(f)-1], f[len(f)-1]
}

// usageHint returns the one-line usage for the verb being typed, or "".
func usageHint(line string) string {
	f := strings.Fields(line)
	if len(f) == 0 {
		return ""
	}
	if s, ok := command.SpecFor(f[0]); ok {
		return s.Usage()
	}
	return ""
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
			if p == "" {
				return ""
			}
		}
	}
	return p
}
