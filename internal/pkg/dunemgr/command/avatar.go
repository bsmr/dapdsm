// Package command — avatar subcommand: per-character export/import/transfer.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/avatar"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
)

const avatarUsage = `usage:
  dunemgr avatar export   <host> <name|fls> [--id]
  dunemgr avatar list     <host>
  dunemgr avatar import   <host> <name|fls> <export-key> [--name <name>] [--id] --confirm
  dunemgr avatar transfer <src-host> <dst-host> <name|fls> [--name <name>] [--check] [--id] --confirm`

func avatarCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		fmt.Fprintln(stderr, avatarUsage)
		return fmt.Errorf("avatar: usage: %w", ErrUsage)
	}
	r := &avatar.Runner{
		DB:      &dbquery.Runner{SSH: c.SSH, Store: c.Store},
		Store:   c.Store,
		DataDir: filepath.Join(c.DataDir, "avatars"),
	}
	switch args[0] {
	case "export":
		return avatarExport(ctx, c, r, args[1:], stdout, stderr)
	case "list":
		return avatarList(r, args[1:], stdout, stderr)
	case "import":
		return avatarImport(ctx, c, r, args[1:], stdout, stderr)
	case "transfer":
		return avatarTransfer(ctx, c, r, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown avatar subcommand %q (want export|list|import|transfer)\n", args[0])
		return fmt.Errorf("avatar: unknown sub %q: %w", args[0], ErrUsage)
	}
}

func avatarExport(ctx context.Context, c *core.Core, r *avatar.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar export <host> <name|fls> [--id]")
		return fmt.Errorf("avatar export: usage: %w", ErrUsage)
	}
	host := args[0]
	useID := hasFlag(args[2:], "--id")
	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	fls, err := resolvePlayerArg(ctx, dbr, host, args[1], useID, stderr)
	if err != nil {
		return err
	}
	rec, err := r.Export(ctx, "cli", host, fls)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "avatar exported\nkey=%s\nname=%s\nlocal=%s\nbytes=%d\n",
		rec.Key(), rec.CharacterName, rec.LocalPath, rec.Bytes)
	return nil
}

func avatarList(r *avatar.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar list <host>")
		return fmt.Errorf("avatar list: usage: %w", ErrUsage)
	}
	rows, err := r.Store.ListExports(args[0])
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "(no exports)")
		return nil
	}
	for _, e := range rows {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%d bytes\t%s\n",
			e.CreatedAt.Format("2006-01-02T15:04Z"), e.FLSID, e.CharacterName, e.Bytes, e.Key())
	}
	return nil
}

func avatarImport(ctx context.Context, c *core.Core, r *avatar.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar import <host> <name|fls> <export-key> [--name <name>] [--id] --confirm")
		return fmt.Errorf("avatar import: usage: %w", ErrUsage)
	}
	host, flsRef, key := args[0], args[1], args[2]
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "character name (defaults to the export record's stored name)")
	confirm := fs.Bool("confirm", false, "explicitly confirm the destructive import")
	useID := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	if err := fs.Parse(args[3:]); err != nil {
		return err
	}
	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	fls, err := resolvePlayerArg(ctx, dbr, host, flsRef, *useID, stderr)
	if err != nil {
		return err
	}
	ctrlID, err := r.Import(ctx, "cli", host, fls, key, *name, *confirm)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "import ok\ncontroller_id=%d\n", ctrlID)
	return nil
}

func avatarTransfer(ctx context.Context, c *core.Core, r *avatar.Runner, args []string, stdout, stderr io.Writer) error {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar transfer <src-host> <dst-host> <name|fls> [--name <name>] [--check] [--id] --confirm")
		return fmt.Errorf("avatar transfer: usage: %w", ErrUsage)
	}
	src, dst, flsRef := args[0], args[1], args[2]
	fs := flag.NewFlagSet("transfer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "character name (defaults to the source player's name)")
	check := fs.Bool("check", false, "dry-run: run pre-flight gates only, change nothing")
	confirm := fs.Bool("confirm", false, "explicitly confirm the destructive transfer")
	useID := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	if err := fs.Parse(args[3:]); err != nil {
		return err
	}
	// Resolve against the source host — that's where the player must exist.
	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	fls, err := resolvePlayerArg(ctx, dbr, src, flsRef, *useID, stderr)
	if err != nil {
		return err
	}
	res, err := r.Transfer(ctx, "cli", src, dst, fls, *name, *check, *confirm)
	if err != nil {
		return err
	}
	if *check {
		printPreflight(stdout, src, dst, res.Report)
		return nil
	}
	fmt.Fprintf(stdout, "transfer ok\nfrom=%s to=%s\ncontroller_id=%d\nexport_key=%s\n",
		src, dst, res.ControllerID, res.ExportKey)
	return nil
}

func printPreflight(w io.Writer, src, dst string, p *avatar.PreflightReport) {
	yes := func(b bool) string {
		if b {
			return "PASS"
		}
		return "FAIL"
	}
	fmt.Fprintln(w, "pre-flight (dry-run, nothing changed):")
	fmt.Fprintf(w, "  %s offline:       %s\n", src, yes(p.SrcOffline))
	fmt.Fprintf(w, "  %s offline:       %s\n", dst, yes(p.DstOffline))
	fmt.Fprintf(w, "  fls exists on src: %s (%s)\n", yes(p.FLSExists), p.CharacterName)
	fmt.Fprintf(w, "  checksum match:    %s (%s / %s)\n", yes(p.ChecksumMatch), p.SrcChecksum, p.DstChecksum)
	if p.OK() {
		fmt.Fprintln(w, "  => transfer would SUCCEED")
	} else {
		fmt.Fprintln(w, "  => transfer would be REJECTED")
	}
}
