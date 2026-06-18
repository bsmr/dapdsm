package command

import (
	"context"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/domain/gamedb"
)

// resolvePlayerArg turns ref (a character name or a FLS id) into a FLS id for a
// player-targeting verb. With useID, ref is taken as a raw FLS (no resolution).
// On an ambiguous name it writes the candidate list to stderr and returns
// ErrUsage; on no match it returns the resolver error wrapped as ErrUsage.
func resolvePlayerArg(ctx context.Context, dbr *gamedb.Runner, host, ref string, useID bool, stderr io.Writer) (string, error) {
	if useID {
		return ref, nil
	}
	fls, ambiguous, err := dbr.ResolvePlayerRef(ctx, host, ref)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return "", fmt.Errorf("resolve player: %w", ErrUsage)
	}
	if len(ambiguous) > 0 {
		formatAmbiguous(stderr, ref, ambiguous)
		return "", fmt.Errorf("ambiguous player %q: %w", ref, ErrUsage)
	}
	return fls, nil
}

// formatAmbiguous prints the candidate players for an ambiguous reference.
func formatAmbiguous(w io.Writer, ref string, players []gamedb.Player) {
	fmt.Fprintf(w, "%q matches %d players — narrow it or pass the fls with --id:\n", ref, len(players))
	for _, p := range players {
		fmt.Fprintf(w, "  %s  %s  (%s)\n", p.FLSID, p.CharacterName, p.OnlineStatus)
	}
}

// hasFlag reports whether tokens contains the exact flag (e.g. "--id").
func hasFlag(tokens []string, flag string) bool {
	for _, t := range tokens {
		if t == flag {
			return true
		}
	}
	return false
}

// stripFlag returns a copy of tokens with all occurrences of the exact flag
// removed. Used to remove flags consumed out-of-band (e.g. --id) before
// passing remaining args to a flag.FlagSet that does not define them.
func stripFlag(tokens []string, flag string) []string {
	out := tokens[:0:0] // reuse capacity but not backing array
	for _, t := range tokens {
		if t != flag {
			out = append(out, t)
		}
	}
	return out
}
