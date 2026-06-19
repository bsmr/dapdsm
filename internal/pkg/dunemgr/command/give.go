package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/gamedb"
	"go.muehmer.eu/dapdsm/pkg/domain/grant"
	"go.muehmer.eu/dapdsm/pkg/domain/mq"
)

const giveUsage = `usage:
  dunemgr give <host> currency <name|fls> <currency-id> <delta> [--check] [--force] [--id]
  dunemgr give <host> item <name|fls> <item-name> <count> [--quality N] [--durability F] [--check] [--id]
  dunemgr give <host> skillpoints <name|fls> <amount> [--check] [--force] [--id]
  dunemgr give <host> xp <name|fls> <amount> --track <Combat|Melee|Mentat|Trooper|Swordmaster|Planetologist|BeneGesserit|Vehicle> [--check] [--id]
  dunemgr give <host> charxp <name|fls> <amount> [--check] [--force] [--id]`

func giveCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, giveUsage)
		return fmt.Errorf("give: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	g := &grant.Granter{
		DB:    &gamedb.Runner{SSH: c.SSH, Store: c.Store},
		MQ:    &mq.Publisher{Exec: c.SSH, Store: c.Store},
		Store: c.Store,
	}

	fs := flag.NewFlagSet("give", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "dry-run: resolve target/presence/backend, change nothing")
	force := fs.Bool("force", false, "apply a DB-only grant even if the player is online")
	quality := fs.Int64("quality", 0, "item quality (DB/offline path only)")
	durability := fs.Float64("durability", 1.0, "item durability (MQ/online path only)")
	id := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	track := fs.String("track", "", "specialization track for `give xp` (required)")

	var req grant.Req
	req.FLS = giveFirst(rest)
	switch sub {
	case "currency":
		if len(rest) < 3 {
			fmt.Fprintln(stderr, giveUsage)
			return fmt.Errorf("give currency: usage: %w", ErrUsage)
		}
		cid, err1 := strconv.Atoi(rest[1])
		delta, err2 := strconv.ParseInt(rest[2], 10, 64)
		if err1 != nil || err2 != nil {
			fmt.Fprintln(stderr, "give currency: <currency-id> and <delta> must be integers")
			return fmt.Errorf("give currency: parse: %w", ErrUsage)
		}
		req.Verb, req.CurrencyID, req.Delta = grant.VerbCurrency, cid, delta
		if err := fs.Parse(rest[3:]); err != nil {
			return err
		}
	case "item":
		if len(rest) < 3 {
			fmt.Fprintln(stderr, giveUsage)
			return fmt.Errorf("give item: usage: %w", ErrUsage)
		}
		count, err := strconv.ParseInt(rest[2], 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "give item: <count> must be an integer")
			return fmt.Errorf("give item: parse: %w", ErrUsage)
		}
		req.Verb, req.Item, req.Count = grant.VerbItem, rest[1], count
		if err := fs.Parse(rest[3:]); err != nil {
			return err
		}
		req.Quality, req.Durability = *quality, *durability
	case "skillpoints":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, giveUsage)
			return fmt.Errorf("give skillpoints: usage: %w", ErrUsage)
		}
		amount, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "give skillpoints: <amount> must be an integer")
			return fmt.Errorf("give skillpoints: parse: %w", ErrUsage)
		}
		req.Verb, req.Amount = grant.VerbSkillpoints, amount
		if err := fs.Parse(rest[2:]); err != nil {
			return err
		}
	case "xp":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, giveUsage)
			return fmt.Errorf("give xp: usage: %w", ErrUsage)
		}
		amount, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "give xp: <amount> must be an integer")
			return fmt.Errorf("give xp: parse: %w", ErrUsage)
		}
		req.Verb, req.XP = grant.VerbXP, amount
		if err := fs.Parse(rest[2:]); err != nil {
			return err
		}
		canon, ok := grant.CanonicalTrack(*track)
		if !ok {
			fmt.Fprintf(stderr, "give xp: --track required, one of %v\n", grant.Tracks())
			return fmt.Errorf("give xp: track: %w", ErrUsage)
		}
		req.Track = canon
	case "charxp":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, giveUsage)
			return fmt.Errorf("give charxp: usage: %w", ErrUsage)
		}
		amount, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil {
			fmt.Fprintln(stderr, "give charxp: <amount> must be an integer")
			return fmt.Errorf("give charxp: parse: %w", ErrUsage)
		}
		req.Verb, req.XP = grant.VerbCharXP, amount
		if err := fs.Parse(rest[2:]); err != nil {
			return err
		}
	default:
		fmt.Fprintf(stderr, "unknown give subcommand %q (want currency|item|skillpoints|xp|charxp)\n", sub)
		return fmt.Errorf("give: unknown sub %q: %w", sub, ErrUsage)
	}
	req.Force = *force

	dbr := &gamedb.Runner{SSH: c.SSH, Store: c.Store}
	resolved, err := resolvePlayerArg(ctx, dbr, host, req.FLS, *id, stderr)
	if err != nil {
		return err
	}
	req.FLS = resolved

	if *check {
		p, err := g.Plan(ctx, host, req)
		if err != nil {
			return err
		}
		presence := "online"
		if p.Offline {
			presence = "offline"
		}
		fmt.Fprintf(stdout, "dry-run (nothing changed):\n  target=%s  presence=%s  backend=%s\n  action=%s\n",
			p.FLS, presence, p.Backend, p.Summary)
		return nil
	}
	res, err := g.Apply(ctx, "cli", host, req)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "grant ok: %s\n", res.Detail)
	return nil
}

// giveFirst returns the first element of s, or "" when empty.
func giveFirst(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}
