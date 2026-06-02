package stats

import (
	"context"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

func TestParseMeminfo(t *testing.T) {
	in := "MemTotal:       16384000 kB\nMemAvailable:    8192000 kB\nMemFree: 100 kB\n"
	tot, avail := parseMeminfo(in)
	if tot != 16384000*1024 || avail != 8192000*1024 {
		t.Fatalf("meminfo parse: tot=%d avail=%d", tot, avail)
	}
}

func TestParseProcStatDelta(t *testing.T) {
	a := "cpu  100 0 50 1000 0 0 0 0\n"
	b := "cpu  110 0 55 1090 0 0 0 0\n"
	pct := cpuPercent(parseProcStat(a), parseProcStat(b))
	if pct < 13 || pct > 16 {
		t.Fatalf("cpu%% ~14.3 expected, got %.1f", pct)
	}
}

func TestParseNetDevDelta(t *testing.T) {
	a := "Inter-|...\n face |...\n  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\n lo: 5 0\n"
	b := "Inter-|...\n face |...\n  eth0: 3000 0 0 0 0 0 0 0 5000 0 0 0 0 0 0 0\n lo: 5 0\n"
	rx, tx := netDelta(parseNetDev(a), parseNetDev(b))
	if rx != 2000 || tx != 3000 {
		t.Fatalf("net delta rx=%d tx=%d", rx, tx)
	}
}

func TestParseDf(t *testing.T) {
	in := "1024000000 512000000 /\n2048000000 1024000000 /var/lib/rancher\n"
	rows := parseDf(in)
	if len(rows) != 2 || rows[0].Mount != "/" || rows[0].UsedBytes != 512000000 {
		t.Fatalf("df parse: %+v", rows)
	}
}

type statRunner struct{ out string }

func (r *statRunner) Run(ctx context.Context, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: r.out, ExitCode: 0}, nil
}
func (r *statRunner) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) (ssh.Result, error) {
	return ssh.Result{Stdout: r.out, ExitCode: 0}, nil
}

func TestCollectParsesBlocks(t *testing.T) {
	sample := "@@LOAD\n0.50 0.40 0.30 1/200 1\n" +
		"@@STAT1\ncpu  100 0 50 1000 0 0 0 0\n" +
		"@@MEM\nMemTotal: 16384000 kB\nMemAvailable: 8192000 kB\n" +
		"@@DF\n1024000000 512000000 /\n" +
		"@@NET1\n  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\n" +
		"@@STAT2\ncpu  110 0 55 1090 0 0 0 0\n" +
		"@@NET2\n  eth0: 3000 0 0 0 0 0 0 0 5000 0 0 0 0 0 0 0\n"
	s := &statRunner{out: sample}
	snap, err := Collect(context.Background(), &ssh.Client{Runner: s}, "h")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Load1 != 0.50 || snap.MemTotal == 0 {
		t.Fatalf("snapshot incomplete: %+v", snap)
	}
	if snap.CPUPercent < 13 || snap.CPUPercent > 16 {
		t.Fatalf("CPUPercent ~14.3 expected from the sample, got %.1f", snap.CPUPercent)
	}
	if len(snap.Disks) != 1 {
		t.Fatalf("disks: %+v", snap.Disks)
	}
}
