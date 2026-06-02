package command

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
)

// playerCmd runs player-lookup sub-commands (search|pos|inspect) against the
// BattleGroup database on the named host via SSH + kubectl exec.
func playerCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr player <host> search <query> [--limit N]")
		fmt.Fprintln(stderr, "       dunemgr player <host> pos <name|fls> [--id]")
		fmt.Fprintln(stderr, "       dunemgr player <host> inspect <name|fls> [--top N] [--raw] [--id]")
		return fmt.Errorf("player: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	r := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	switch sub {
	case "search":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr player <host> search <query> [--limit N]")
			return fmt.Errorf("player search: usage: %w", ErrUsage)
		}
		query, limit := parseSearchArgs(rest)
		players, err := r.PlayerSearch(ctx, host, query, limit)
		if err != nil {
			return err
		}
		printPlayers(stdout, players)
		return nil
	case "pos":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr player <host> pos <name|fls> [--id]")
			return fmt.Errorf("player pos: usage: %w", ErrUsage)
		}
		flsID, err := resolvePlayerArg(ctx, r, host, rest[0], hasFlag(rest[1:], "--id"), stderr)
		if err != nil {
			return err
		}
		pos, err := r.PlayerPosition(ctx, host, flsID)
		if err != nil {
			return err
		}
		if pos == nil {
			fmt.Fprintln(stdout, "offline / no live pawn")
			return nil
		}
		printPos(stdout, pos)
		return nil
	case "inspect":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr player <host> inspect <name|fls> [--top N] [--raw] [--id]")
			return fmt.Errorf("player inspect: usage: %w", ErrUsage)
		}
		ref := rest[0]
		top := 10
		raw := false
		useID := false
		for i := 1; i < len(rest); i++ {
			switch rest[i] {
			case "--raw":
				raw = true
			case "--id":
				useID = true
			case "--top":
				if i+1 < len(rest) {
					if n, e := strconv.Atoi(rest[i+1]); e == nil {
						top = n
					}
					i++
				}
			}
		}
		flsID, err := resolvePlayerArg(ctx, r, host, ref, useID, stderr)
		if err != nil {
			return err
		}
		d, err := r.PlayerInspect(ctx, host, flsID, top, raw)
		if err != nil {
			return err
		}
		printInspect(stdout, d)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown player subcommand %q (want search|pos|inspect)\n", sub)
		return fmt.Errorf("player: unknown sub %q: %w", sub, ErrUsage)
	}
}

// parseSearchArgs extracts the query string and optional --limit N from the
// remaining args after "search". Returns query and limit (0 = use default).
func parseSearchArgs(args []string) (query string, limit int) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--limit" && i+1 < len(args):
			limit, _ = strconv.Atoi(args[i+1])
			i++
		case !strings.HasPrefix(args[i], "--"):
			query = args[i]
		}
	}
	return query, limit
}

// printPlayers renders a tabular view of the search results to w.
func printPlayers(w io.Writer, players []dbquery.Player) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FLS-ID\tNAME\tSTATUS\tLAST-SEEN")
	for _, p := range players {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			p.FLSID, p.CharacterName, p.OnlineStatus, p.LastSeen)
	}
	_ = tw.Flush()
}

// printInspect renders a player detail to w. It delegates the main body to
// FormatInspect and appends the RawComponents block when present.
func printInspect(w io.Writer, d *dbquery.PlayerDetail) {
	fmt.Fprint(w, FormatInspect(d))
	if d != nil && d.RawComponents != "" {
		fmt.Fprintf(w, "  raw components:\n%s\n", d.RawComponents)
	}
}

// printPos renders a position row to w.
func printPos(w io.Writer, pos *dbquery.Pos) {
	fmt.Fprintf(w, "x=%.3f  y=%.3f  z=%.3f\n", pos.X, pos.Y, pos.Z)
	dim := "-"
	if pos.Dimension != nil {
		dim = strconv.Itoa(*pos.Dimension)
	}
	part := "-"
	if pos.Partition != nil {
		part = strconv.FormatInt(*pos.Partition, 10)
	}
	fmt.Fprintf(w, "dimension=%s  partition=%s  class=%s\n", dim, part, pos.Class)
}
