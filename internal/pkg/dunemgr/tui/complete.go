package tui

import (
	"sort"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/command"
)

// helpVerb is the TUI built-in verb (handled in the model, not the dispatcher).
const helpVerb = "help"

// suggest returns completion candidates for the current (last) token of line,
// prefix-filtered. hosts supplies argHost candidates; selHost is the selected
// host; cache is the live per-host player-name pool.
func suggest(line string, hosts []string, selHost string, cache map[string][]string) []string {
	tokens, cur := splitCurrent(line)
	if len(tokens) == 0 {
		var pool []string
		for _, s := range command.Specs() {
			pool = append(pool, s.Verb)
		}
		pool = append(pool, helpVerb)
		sort.Strings(pool)
		return prefixFilter(pool, cur)
	}
	spec, ok := command.SpecFor(tokens[0])
	if !ok {
		return nil
	}
	norm := normalizeTokens(spec, tokens, selHost, hosts)
	argPos := len(norm) - 1
	if spec.IsCatalogPos(argPos, norm...) && cur == "" {
		return nil // a catalog can have thousands of entries
	}
	if spec.IsPlayerPos(argPos, norm...) {
		return suggestPlayers(line, selHost, cache)
	}
	return prefixFilter(spec.Candidates(argPos, hosts, norm...), cur)
}

// normalizeTokens inserts the selected host at index 1 when spec is host-first
// and no explicit host was typed, so slot math (argPos, sub-verb lookup) is
// identical whether the host is explicit or implied. tokens are the COMPLETED
// tokens (verb + finished args, excluding the in-progress token). The visible
// input is never changed by this — only the completion math uses the
// normalised copy.
func normalizeTokens(spec command.Spec, tokens []string, selHost string, hosts []string) []string {
	if selHost == "" || !spec.FirstArgIsHost() {
		return tokens
	}
	if len(tokens) >= 2 && isKnownHost(tokens[1], hosts) {
		return tokens // explicit host already present
	}
	out := make([]string, 0, len(tokens)+1)
	out = append(out, tokens[0], selHost)
	out = append(out, tokens[1:]...)
	return out
}

func prefixFilter(pool []string, cur string) []string {
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

// usageHint returns the flag-bearing long-form grammar for the verb being
// typed, or "". Uses command.UsageLong to include flags and sub-verb matching.
func usageHint(line string) string {
	f := strings.Fields(line)
	if len(f) == 0 {
		return ""
	}
	return command.UsageLong(f)
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
