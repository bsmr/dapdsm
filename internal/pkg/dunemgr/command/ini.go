// Package command — ini subcommand: curated gameplay-settings editor.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/gameini"
)

func iniCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, iniUsage)
		return fmt.Errorf("ini: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	r := &gameini.Runner{SSH: c.SSH}
	switch sub {
	case "list":
		rows, err := r.List(ctx, host)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "KEY\tVALUE")
		for _, kv := range rows {
			v := kv.Value
			if v == "" {
				v = "(unset)"
			}
			fmt.Fprintf(tw, "%s\t%s\n", kv.Key, v)
		}
		_ = tw.Flush()
		return nil
	case "get":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr ini <host> get <key>")
			return fmt.Errorf("ini get: usage: %w", ErrUsage)
		}
		v, err := r.Get(ctx, host, rest[0])
		if err != nil {
			return err
		}
		if v == "" {
			v = "(unset)"
		}
		fmt.Fprintf(stdout, "%s = %s\n", rest[0], v)
		return nil
	case "set":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "usage: dunemgr ini <host> set <key> <value> [--apply] [--restart]")
			return fmt.Errorf("ini set: usage: %w", ErrUsage)
		}
		key, value, flags := rest[0], rest[1], rest[2:]
		fs := flag.NewFlagSet("set", flag.ContinueOnError)
		fs.SetOutput(stderr)
		apply := fs.Bool("apply", false, "run battlegroup apply-default-usersettings after the edit")
		restart := fs.Bool("restart", false, "run battlegroup restart after the edit")
		if err := fs.Parse(flags); err != nil {
			return err
		}
		if err := r.Set(ctx, host, key, value, *apply, *restart); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "set %s = %s\n", key, value)
		if !*apply && !*restart {
			fmt.Fprintln(stdout, "note: not applied — run with --apply --restart, or `dunemgr lifecycle <host> restart`, for it to take effect")
		}
		return nil
	default:
		fmt.Fprintf(stderr, "unknown ini subcommand %q (want list|get|set)\n", sub)
		return fmt.Errorf("ini: unknown sub %q: %w", sub, ErrUsage)
	}
}

const iniUsage = `usage:
  dunemgr ini <host> list
  dunemgr ini <host> get <key>
  dunemgr ini <host> set <key> <value> [--apply] [--restart]`
