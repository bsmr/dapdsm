// Package stats collects at-a-glance node telemetry (CPU/mem/disk/net) from a
// host over SSH by sampling /proc + df. Pure parsers + a single-round-trip
// Collect; no metrics-server dependency.
package stats

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// Snapshot is one telemetry reading.
type Snapshot struct {
	Load1, Load5, Load15 float64
	CPUPercent           float64
	MemTotal, MemAvail   int64
	Disks                []Disk
	NetRXBytesPerSec     int64
	NetTXBytesPerSec     int64
}

// Disk is one filesystem's usage.
type Disk struct {
	Mount      string
	TotalBytes int64
	UsedBytes  int64
}

type cpuSample struct{ busy, total int64 }

func parseProcStat(s string) cpuSample {
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) >= 8 && f[0] == "cpu" {
			var vals []int64
			for _, v := range f[1:] {
				n, _ := strconv.ParseInt(v, 10, 64)
				vals = append(vals, n)
			}
			var total int64
			for _, v := range vals {
				total += v
			}
			idle := vals[3]
			if len(vals) > 4 {
				idle += vals[4]
			}
			return cpuSample{busy: total - idle, total: total}
		}
	}
	return cpuSample{}
}

func cpuPercent(a, b cpuSample) float64 {
	dt := b.total - a.total
	if dt <= 0 {
		return 0
	}
	return float64(b.busy-a.busy) / float64(dt) * 100
}

func parseMeminfo(s string) (total, avail int64) {
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			kb, _ := strconv.ParseInt(f[1], 10, 64)
			switch f[0] {
			case "MemTotal:":
				total = kb * 1024
			case "MemAvailable:":
				avail = kb * 1024
			}
		}
	}
	return total, avail
}

type netSample map[string][2]int64

func parseNetDev(s string) netSample {
	out := netSample{}
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		f := strings.Fields(parts[1])
		if iface == "" || iface == "lo" || len(f) < 9 {
			continue
		}
		rx, _ := strconv.ParseInt(f[0], 10, 64)
		tx, _ := strconv.ParseInt(f[8], 10, 64)
		out[iface] = [2]int64{rx, tx}
	}
	return out
}

func netDelta(a, b netSample) (rx, tx int64) {
	for iface, bv := range b {
		if av, ok := a[iface]; ok {
			rx += bv[0] - av[0]
			tx += bv[1] - av[1]
		}
	}
	return rx, tx
}

func parseDf(s string) []Disk {
	var out []Disk
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		total, _ := strconv.ParseInt(f[0], 10, 64)
		used, _ := strconv.ParseInt(f[1], 10, 64)
		out = append(out, Disk{TotalBytes: total, UsedBytes: used, Mount: f[2]})
	}
	return out
}

const collectScript = `echo @@LOAD; cat /proc/loadavg; ` +
	`echo @@STAT1; grep '^cpu ' /proc/stat; ` +
	`echo @@MEM; cat /proc/meminfo; ` +
	`echo @@DF; df -B1 --output=size,used,target -x tmpfs -x devtmpfs 2>/dev/null | tail -n +2; ` +
	`echo @@NET1; cat /proc/net/dev; ` +
	`sleep 1; ` +
	`echo @@STAT2; grep '^cpu ' /proc/stat; ` +
	`echo @@NET2; cat /proc/net/dev`

// Collect gathers one Snapshot from host over SSH.
func Collect(ctx context.Context, sshc *ssh.Client, host string) (*Snapshot, error) {
	res, err := sshc.Run(ctx, host, "sh", "-lc", collectScript)
	if err != nil {
		return nil, fmt.Errorf("stats collect: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("stats collect: exit %d: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	blocks := splitBlocks(res.Stdout)
	snap := &Snapshot{}
	if lf := strings.Fields(blocks["LOAD"]); len(lf) >= 3 {
		snap.Load1, _ = strconv.ParseFloat(lf[0], 64)
		snap.Load5, _ = strconv.ParseFloat(lf[1], 64)
		snap.Load15, _ = strconv.ParseFloat(lf[2], 64)
	}
	snap.CPUPercent = cpuPercent(parseProcStat(blocks["STAT1"]), parseProcStat(blocks["STAT2"]))
	snap.MemTotal, snap.MemAvail = parseMeminfo(blocks["MEM"])
	snap.Disks = parseDf(blocks["DF"])
	snap.NetRXBytesPerSec, snap.NetTXBytesPerSec = netDelta(parseNetDev(blocks["NET1"]), parseNetDev(blocks["NET2"]))
	return snap, nil
}

func splitBlocks(out string) map[string]string {
	blocks := map[string]string{}
	cur := ""
	var b strings.Builder
	flush := func() {
		if cur != "" {
			blocks[cur] = b.String()
		}
		b.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "@@") {
			flush()
			cur = strings.TrimPrefix(line, "@@")
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	flush()
	return blocks
}
