// Package command — admin subcommand.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/gamedb"
	"go.muehmer.eu/dapdsm/pkg/domain/mq"
)

// adminCmd dispatches in-game admin commands to the BattleGroup MQ broker.
//
// Usage:
//
//	dunemgr admin <host> <verb> <name|fls|*> [positional name] [flags] [--id]
//
// All available verbs are listed in admin.Verbs(). Destructive verbs (kick,
// clean, reset) and wildcard player "*" require --confirm.
func adminCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		printAdminUsage(stderr)
		return fmt.Errorf("admin: usage: %w", ErrUsage)
	}
	host, verb, playerID, rest := args[0], args[1], args[2], args[3:]

	if !admin.KnownVerb(verb) {
		fmt.Fprintf(stderr, "unknown admin verb %q (want %s)\n", verb, strings.Join(admin.Verbs(), "|"))
		printAdminUsage(stderr)
		return fmt.Errorf("admin: unknown verb %q: %w", verb, ErrUsage)
	}

	// Extract --id before parseAdminFlags so it is not rejected as unknown.
	useID := hasFlag(rest, "--id")
	verbRest := stripFlag(rest, "--id")

	fields, confirm, err := parseAdminFlags(verb, verbRest, stderr)
	if err != nil {
		return err
	}

	// Resolve the player reference unless it is the wildcard "*" or --id is set.
	if playerID != "*" {
		dbr := &gamedb.Runner{SSH: c.SSH, Store: c.Store}
		playerID, err = resolvePlayerArg(ctx, dbr, host, playerID, useID, stderr)
		if err != nil {
			return err
		}
	}

	r := &admin.Runner{MQ: &mq.Publisher{Exec: c.SSH, Store: c.Store}}
	res, err := r.Run(ctx, "cli", host, verb, playerID, fields, confirm)
	if err != nil {
		return err
	}
	if res.OK {
		fmt.Fprintf(stdout, "admin %s %s: published ok\n", verb, playerID)
	} else {
		fmt.Fprintf(stdout, "admin %s %s: publish FAILED\n%s\n", verb, playerID, res.RawOutput)
	}
	return nil
}

// parseAdminFlags parses the per-verb positional and flag arguments.
// Returns the field map, the --confirm flag value, and any error.
func parseAdminFlags(verb string, args []string, stderr io.Writer) (map[string]string, bool, error) {
	fields := map[string]string{}
	fs := flag.NewFlagSet("admin "+verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	confirm := fs.Bool("confirm", false, "required for destructive verbs and wildcard player (*)")

	switch verb {
	case "item":
		var name string
		name, args = popPositional(args) // ItemName before flags (flag.Parse stops at the first positional)
		qty := fs.String("qty", "", "item quantity (default 1)")
		dur := fs.String("durability", "", "item durability 0–1 (default 1.0)")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if name != "" {
			fields["ItemName"] = name
		}
		if *qty != "" {
			fields["Quantity"] = *qty
		}
		if *dur != "" {
			fields["Durability"] = *dur
		}

	case "water":
		amount := fs.String("amount", "", "water amount (default 1000000)")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if *amount != "" {
			fields["WaterAmount"] = *amount
		}

	case "xp":
		xp := fs.String("xp", "", "XP amount (default 1000)")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if *xp != "" {
			fields["Experience"] = *xp
		}

	case "skill":
		var name string
		name, args = popPositional(args) // Module before flags
		level := fs.String("level", "", "module level (default 1)")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if name != "" {
			fields["Module"] = name
		}
		if *level != "" {
			fields["Level"] = *level
		}

	case "skillpoints":
		points := fs.String("points", "", "unspent skill points to SET (absolute). Prefer the additive: give skillpoints <player> N")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if *points == "" {
			fmt.Fprintln(stderr, "admin skillpoints: --points N required (or use the presence-aware additive: give skillpoints <player> N)")
			return nil, false, fmt.Errorf("admin skillpoints: --points required: %w", ErrUsage)
		}
		fields["SkillPoints"] = *points

	case "vehicle":
		var name string
		name, args = popPositional(args) // ClassName before flags
		tmpl := fs.String("template", "", "vehicle template name (required)")
		x := fs.String("x", "", "X coordinate (required)")
		y := fs.String("y", "", "Y coordinate (required)")
		z := fs.String("z", "", "Z coordinate (required)")
		rotation := fs.String("rotation", "", "rotation angle")
		persistent := fs.String("persistent", "", "persistent flag (default 1.0)")
		faction := fs.String("faction", "", "faction string")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if name != "" {
			fields["ClassName"] = name
		}
		if *tmpl != "" {
			fields["TemplateName"] = *tmpl
		}
		if *x != "" {
			fields["X"] = *x
		}
		if *y != "" {
			fields["Y"] = *y
		}
		if *z != "" {
			fields["Z"] = *z
		}
		if *rotation != "" {
			fields["Rotation"] = *rotation
		}
		if *persistent != "" {
			fields["Persistent"] = *persistent
		}
		if *faction != "" {
			fields["Faction"] = *faction
		}

	case "teleport", "teleport-exact":
		x := fs.String("x", "", "X coordinate (required)")
		y := fs.String("y", "", "Y coordinate (required)")
		z := fs.String("z", "", "Z coordinate (required)")
		yaw := fs.String("yaw", "", "yaw angle")
		camPitch := fs.String("cam-pitch", "", "camera pitch")
		camYaw := fs.String("cam-yaw", "", "camera yaw")
		camRoll := fs.String("cam-roll", "", "camera roll")
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		if *x != "" {
			fields["X"] = *x
		}
		if *y != "" {
			fields["Y"] = *y
		}
		if *z != "" {
			fields["Z"] = *z
		}
		if *yaw != "" {
			fields["Yaw"] = *yaw
		}
		if *camPitch != "" {
			fields["CamPitch"] = *camPitch
		}
		if *camYaw != "" {
			fields["CamYaw"] = *camYaw
		}
		if *camRoll != "" {
			fields["CamRoll"] = *camRoll
		}

	case "kick", "clean", "reset":
		if err := fs.Parse(args); err != nil {
			return nil, false, err
		}
		// No additional fields; --confirm handled below.

	default:
		// Already validated above; unreachable.
		return nil, false, fmt.Errorf("admin: unknown verb %q: %w", verb, ErrUsage)
	}

	return fields, *confirm, nil
}

// popPositional splits off a leading non-flag argument (a positional name like
// ItemName/Module/ClassName). Go's flag package stops parsing at the first
// positional, so the name must be removed before fs.Parse for the trailing flags
// to be recognised.
func popPositional(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

// printAdminUsage writes the usage hint for the admin verb.
func printAdminUsage(w io.Writer) {
	verbs := strings.Join(admin.Verbs(), "|")
	fmt.Fprintf(w, "usage: dunemgr admin <host> <%s> <name|fls|*> [args] [--id] [--confirm]\n", verbs)
}
