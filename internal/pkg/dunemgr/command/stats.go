// Package command — stats subcommand: node telemetry.
package command

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/stats"
)

func statsCmd(ctx context.Context, c *core.Core, args []string, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: dunemgr stats <host>")
		return fmt.Errorf("stats: usage: %w", ErrUsage)
	}
	snap, err := stats.Collect(ctx, c.SSH, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "host %s\n", args[0])
	fmt.Fprintf(stdout, "  load:  %.2f %.2f %.2f   cpu: %.1f%%\n", snap.Load1, snap.Load5, snap.Load15, snap.CPUPercent)
	used := snap.MemTotal - snap.MemAvail
	fmt.Fprintf(stdout, "  mem:   %s used / %s total\n", formatBytes(used), formatBytes(snap.MemTotal))
	fmt.Fprintf(stdout, "  net:   rx %s/s  tx %s/s\n", formatBytes(snap.NetRXBytesPerSec), formatBytes(snap.NetTXBytesPerSec))
	if len(snap.Disks) > 0 {
		tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  MOUNT\tUSED\tTOTAL")
		for _, d := range snap.Disks {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", d.Mount, formatBytes(d.UsedBytes), formatBytes(d.TotalBytes))
		}
		_ = tw.Flush()
	}
	return nil
}

// formatBytes renders a byte count in binary units.
func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	f := float64(n)
	for _, u := range units {
		f /= 1024
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
	}
	return fmt.Sprintf("%.1f EiB", f/1024)
}
