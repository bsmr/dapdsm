package server

import (
	"fmt"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
)

// lifecycleView is the template data for the lifecycle partial.
type lifecycleView struct {
	Host          string
	Waiting       bool // true: cache empty / never probed
	BGState       string
	DBPhase       string
	DirectorPhase string
	Size          int
	Uptime        string // "" when StartedAt is zero
	Error         string
	Servers       []battlegroup.ServerStatus
}

// buildLifecycleView turns a cached snapshot into template data. found
// is false when the store had no snapshot for this host (never probed).
func buildLifecycleView(host string, snap store.StatusSnapshot, found bool, now time.Time) lifecycleView {
	if !found {
		return lifecycleView{Host: host, Waiting: true}
	}
	v := lifecycleView{
		Host:          host,
		BGState:       snap.BGState,
		DBPhase:       snap.Detail.DBPhase,
		DirectorPhase: snap.Detail.DirectorPhase,
		Size:          snap.Detail.Size,
		Error:         snap.Error,
		Servers:       snap.Detail.Servers,
	}
	if !snap.Detail.StartedAt.IsZero() {
		v.Uptime = formatUptime(now.Sub(snap.Detail.StartedAt))
	}
	return v
}

// formatUptime renders a duration as compact "<h>h<m>m" (or "<m>m" under
// an hour), flooring to the minute.
func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
