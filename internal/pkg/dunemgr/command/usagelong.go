// Package command — UsageLong exposes the flag-bearing long-form usage grammar.

package command

import "strings"

// usageLongSrc maps a verb to its multi-line long-form usage constant. These
// constants are the single source of truth for the flag-bearing grammar; this
// map only references them (no duplication).
var usageLongSrc = map[string]string{
	"avatar": avatarUsage,
	"give":   giveUsage,
	"ini":    iniUsage,
	"item":   itemUsage,
}

// grammarLines extracts the grammar lines from a long-usage constant: the lines
// that describe an invocation (they start with "dunemgr "), with the "dunemgr "
// prefix and any trailing "# ..." comment stripped. Prose/explanation lines are
// dropped. "dunemgr " must be the first non-space content on the line so that
// prose sentences that reference another sub-command (e.g. "...Find ids via:
// dunemgr player ...") are not mistaken for grammar lines.
func grammarLines(raw string) []string {
	// Grammar lines must start with "dunemgr " after only leading whitespace;
	// each long-usage const therefore keeps "usage:" on its own line so the
	// first invocation is not skipped (see giveUsage).
	var out []string
	for _, ln := range strings.Split(raw, "\n") {
		i := strings.Index(ln, "dunemgr ")
		if i < 0 || strings.TrimSpace(ln[:i]) != "" {
			continue
		}
		g := strings.TrimSpace(ln[i+len("dunemgr "):])
		if c := strings.Index(g, " #"); c >= 0 {
			g = strings.TrimSpace(g[:c])
		}
		if g != "" {
			out = append(out, g)
		}
	}
	return out
}

// UsageLong returns the flag-bearing grammar for the verb in argv[0]. When a
// sub-verb is typed (argv[1:]), it returns the single grammar line that best
// matches the typed tokens; otherwise all grammar lines for the verb. Verbs
// without a long-usage constant fall back to Spec.Usage(); unknown verbs → "".
func UsageLong(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	verb := argv[0]
	raw, ok := usageLongSrc[verb]
	if !ok {
		if s, ok := SpecFor(verb); ok {
			return s.Usage()
		}
		return ""
	}
	lines := grammarLines(raw)
	if len(lines) == 0 {
		if s, ok := SpecFor(verb); ok {
			return s.Usage()
		}
		return ""
	}
	if len(argv) > 1 {
		best, bestScore := "", 0
		for _, ln := range lines {
			words := strings.Fields(ln)
			score := 0
			for _, tok := range argv[1:] {
				for _, w := range words {
					if w == tok {
						score++
						break
					}
				}
			}
			if score > bestScore {
				best, bestScore = ln, score
			}
		}
		if bestScore > 0 {
			return best
		}
	}
	return strings.Join(lines, "\n")
}
