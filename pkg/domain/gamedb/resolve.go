package gamedb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// flsShapeRE matches a 16-hex-char Funcom FLS id (accounts."user").
var flsShapeRE = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)

// ResolvePlayerRef turns a reference (a FLS id or a character name) into a FLS
// id. A 16-hex ref is returned as-is (no DB hit). Otherwise it prefix-searches
// character names: an exact (case-insensitive) name match wins; else a single
// match is used; multiple matches are returned in ambiguous (no fls, no error);
// no match is an error. Read-only.
func (r *Runner) ResolvePlayerRef(ctx context.Context, host, ref string) (fls string, ambiguous []Player, err error) {
	if flsShapeRE.MatchString(ref) {
		return ref, nil, nil
	}
	rows, err := r.PlayerSearch(ctx, host, ref+"%", 200)
	if err != nil {
		return "", nil, fmt.Errorf("resolve player %q: %w", ref, err)
	}
	var exact []Player
	for _, p := range rows {
		if strings.EqualFold(p.CharacterName, ref) {
			exact = append(exact, p)
		}
	}
	switch {
	case len(exact) == 1:
		return exact[0].FLSID, nil, nil
	case len(exact) > 1:
		return "", exact, nil
	case len(rows) == 1:
		return rows[0].FLSID, nil, nil
	case len(rows) > 1:
		return "", rows, nil
	default:
		return "", nil, fmt.Errorf("no player matching %q on %s", ref, host)
	}
}
