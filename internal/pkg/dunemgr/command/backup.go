// Package command — backup subcommand.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/backup"
)

// backupCmd creates, lists, or restores BattleGroup DB backups on a host.
// Sub-commands: create, list, restore.
func backupCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: dunemgr backup <host> <bg> <create|list|restore> [args...]")
		return fmt.Errorf("backup: usage: %w", ErrUsage)
	}
	host, bg, sub, rest := args[0], args[1], args[2], args[3:]
	dir := c.BackupDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	r := &backup.Runner{SSH: c.SSH, Store: c.Store, DataDir: dir}
	switch sub {
	case "create":
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr backup <host> <bg> create <name>")
			return fmt.Errorf("backup create: usage: %w", ErrUsage)
		}
		rec, err := r.Create(ctx, "cli", host, bg, rest[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "backup created\nkey=%s\nlocal=%s\nbytes=%d\n",
			rec.Key(), rec.LocalPath, rec.Bytes)
		return nil
	case "list":
		rows, err := r.List(ctx, host, bg)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			fmt.Fprintln(stdout, "(no backups)")
			return nil
		}
		for _, b := range rows {
			fmt.Fprintf(stdout, "%s\t%s\t%d bytes\t%s\n",
				b.CreatedAt.Format("2006-01-02T15:04Z"), b.Name, b.Bytes, b.Key())
		}
		return nil
	case "restore":
		fs := flag.NewFlagSet("restore", flag.ContinueOnError)
		fs.SetOutput(stderr)
		confirm := fs.Bool("confirm", false, "explicitly confirm destructive restore")
		if len(rest) < 1 {
			fmt.Fprintln(stderr, "usage: dunemgr backup <host> <bg> restore <key> --confirm")
			return fmt.Errorf("backup restore: usage: %w", ErrUsage)
		}
		key := rest[0]
		if err := fs.Parse(rest[1:]); err != nil {
			return err
		}
		if !*confirm {
			fmt.Fprintln(stderr, "restore is destructive — pass --confirm to proceed")
			return fmt.Errorf("backup restore: missing --confirm: %w", ErrUsage)
		}
		if err := r.Restore(ctx, "cli", key, true); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "restore ok")
		return nil
	default:
		fmt.Fprintf(stderr, "unknown backup subcommand %q (want create|list|restore)\n", sub)
		return fmt.Errorf("backup: unknown sub %q: %w", sub, ErrUsage)
	}
}
