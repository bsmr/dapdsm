// Package cli — host subcommand.
package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/hostpool"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/probe"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// hostCmd manages the host pool (list|add|rm|probe).
// Diagnostic and usage text is written to stderr; real output (tables, probe
// results) goes to stdout. Usage/validation errors are wrapped with ErrUsage.
func hostCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: dunemgr host <add|list|probe|rm> ...: %w", ErrUsage)
	}
	switch args[0] {
	case "list":
		return runHostList(stdout)
	case "add":
		return runHostAdd(ctx, stdout, stderr, args[1:])
	case "rm":
		return runHostRm(stdout, args[1:])
	case "probe":
		return runHostProbe(ctx, stdout, stderr, args[1:])
	default:
		return fmt.Errorf("unknown host subcommand %q: %w", args[0], ErrUsage)
	}
}

func runHostList(stdout io.Writer) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	all, err := s.ListHosts()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSSH-ALIAS")
	for _, h := range all {
		fmt.Fprintf(tw, "%s\t%s\n", h.Name, h.SSHAlias)
	}
	return tw.Flush()
}

func runHostAdd(ctx context.Context, stdout, _ io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dunemgr host add <name> [--ssh-alias=<alias>]: %w", ErrUsage)
	}
	name := args[0]
	alias := name
	for _, a := range args[1:] {
		const p = "--ssh-alias="
		if len(a) > len(p) && a[:len(p)] == p {
			alias = a[len(p):]
		}
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	m := &hostpool.Manager{Store: s}
	if err := m.Register(ctx, name, alias); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "added host %q (ssh alias %q)\n", name, alias)
	return nil
}

func runHostRm(stdout io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dunemgr host rm <name>: %w", ErrUsage)
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	m := &hostpool.Manager{Store: s}
	if err := m.Delete(args[0]); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "removed host %q\n", args[0])
	return nil
}

func runHostProbe(ctx context.Context, stdout, _ io.Writer, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: dunemgr host probe <name>: %w", ErrUsage)
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	snap, err := probe.Probe(ctx, s, ssh.NewClient(), args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "host:    %s\nstate:   %s\npods:    %d/%d\nprobed:  %s\n",
		snap.Host, snap.BGState, snap.PodReady, snap.PodTotal, snap.ProbedAt.Format(time.RFC3339))
	if snap.Error != "" {
		fmt.Fprintf(stdout, "error:   %s\n", snap.Error)
	}
	return nil
}
