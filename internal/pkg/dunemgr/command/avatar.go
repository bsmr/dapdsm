// Package command — avatar subcommand: per-character export/import/transfer.
package command

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/pkg/domain/avatar"
	"go.muehmer.eu/dapdsm/pkg/domain/dbquery"
)

const avatarUsage = `usage:
  dunemgr avatar <host> export   <name|fls> [--id]
  dunemgr avatar <host> list                          # server avatars (joinable characters)
  dunemgr avatar <host> exports                        # local export records (for import keys)
  dunemgr avatar <host> import   <name|fls> <export-key> [--name <name>] [--id] --confirm
  dunemgr avatar <src-host> transfer <dst-host> <name|fls> [--name <name>] [--check] [--id] --confirm`

func avatarRunner(c *core.Core) *avatar.Runner {
	return &avatar.Runner{
		DB:      &dbquery.Runner{SSH: c.SSH, Store: c.Store},
		Store:   c.Store,
		DataDir: filepath.Join(c.DataDir, "avatars"),
	}
}

func avatarCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 2 {
		fmt.Fprintln(stderr, avatarUsage)
		return fmt.Errorf("avatar: usage: %w", ErrUsage)
	}
	host, sub, rest := args[0], args[1], args[2:]
	switch sub {
	case "list":
		return avatarListServer(ctx, c, host, stdout, stderr)
	case "exports":
		return avatarExportsLocal(avatarRunner(c), host, stdout, stderr)
	case "export":
		return avatarExport(ctx, c, avatarRunner(c), host, rest, stdout, stderr)
	case "import":
		return avatarImport(ctx, c, avatarRunner(c), host, rest, stdout, stderr)
	case "transfer":
		return avatarTransfer(ctx, c, avatarRunner(c), host, rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown avatar subcommand %q (want export|list|exports|import|transfer)\n", sub)
		return fmt.Errorf("avatar: unknown sub %q: %w", sub, ErrUsage)
	}
}

func avatarExport(ctx context.Context, c *core.Core, r *avatar.Runner, host string, rest []string, stdout, stderr io.Writer) error {
	if len(rest) < 1 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar <host> export <name|fls> [--id]")
		return fmt.Errorf("avatar export: usage: %w", ErrUsage)
	}
	useID := hasFlag(rest[1:], "--id")
	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	fls, err := resolvePlayerArg(ctx, dbr, host, rest[0], useID, stderr)
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

// avatarListServer lists the joinable characters on host (server-side, read-only).
func avatarListServer(ctx context.Context, c *core.Core, host string, stdout, stderr io.Writer) error {
	dbr := &dbquery.Runner{SSH: c.SSH, Store: c.Store}
	players, err := dbr.PlayerSearch(ctx, host, "%", 200)
	if err != nil {
		return err
	}
	if len(players) == 0 {
		fmt.Fprintln(stdout, "(no avatars on server)")
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tFLS-ID\tSTATUS\tLAST-SEEN")
	for _, p := range players {
		last := p.LastSeen
		if last == "" {
			last = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", p.CharacterName, p.FLSID, p.OnlineStatus, last)
	}
	return tw.Flush()
}

// avatarExportsLocal lists the locally-stored export records for host.
func avatarExportsLocal(r *avatar.Runner, host string, stdout, stderr io.Writer) error {
	rows, err := r.Store.ListExports(host)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "(no exports)")
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CREATED\tFLS-ID\tNAME\tBYTES\tKEY")
	for _, e := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d bytes\t%s\n",
			e.CreatedAt.Format("2006-01-02T15:04Z"), e.FLSID, e.CharacterName, e.Bytes, e.Key())
	}
	return tw.Flush()
}

func avatarImport(ctx context.Context, c *core.Core, r *avatar.Runner, host string, rest []string, stdout, stderr io.Writer) error {
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar <host> import <name|fls> <export-key> [--name <name>] [--id] --confirm")
		return fmt.Errorf("avatar import: usage: %w", ErrUsage)
	}
	flsRef, key := rest[0], rest[1]
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "character name (defaults to the export record's stored name)")
	confirm := fs.Bool("confirm", false, "explicitly confirm the destructive import")
	useID := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	if err := fs.Parse(rest[2:]); err != nil {
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

func avatarTransfer(ctx context.Context, c *core.Core, r *avatar.Runner, src string, rest []string, stdout, stderr io.Writer) error {
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "usage: dunemgr avatar <src-host> transfer <dst-host> <name|fls> [--name <name>] [--check] [--id] --confirm")
		return fmt.Errorf("avatar transfer: usage: %w", ErrUsage)
	}
	dst, flsRef := rest[0], rest[1]
	fs := flag.NewFlagSet("transfer", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "character name (defaults to the source player's name)")
	check := fs.Bool("check", false, "dry-run: run pre-flight gates only, change nothing")
	confirm := fs.Bool("confirm", false, "explicitly confirm the destructive transfer")
	useID := fs.Bool("id", false, "treat <player> as a raw FLS id, skip name resolution")
	if err := fs.Parse(rest[2:]); err != nil {
		return err
	}
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
