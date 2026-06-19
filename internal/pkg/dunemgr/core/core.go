// Package core builds and owns the shared dependencies and background
// routines of dunemgr — the bbolt store, SSH client, the SSE hub + status
// poller, and the schedule manager — so every frontend (CLI dispatcher,
// web UI, TUI) is constructed the same way from one Open call. bbolt permits
// a single open per file, so a process that holds Core open for the poller
// must inject Core.Store into command handlers rather than re-opening it.
package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
	"go.muehmer.eu/dapdsm/pkg/domain/broadcast"
	"go.muehmer.eu/dapdsm/pkg/domain/lifecycle"
	"go.muehmer.eu/dapdsm/pkg/domain/probe"
	"go.muehmer.eu/dapdsm/pkg/domain/schedule"
	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// ssePub adapts an *sse.Hub to schedule.EventPublisher.
type ssePub struct{ h *sse.Hub }

func (s ssePub) Publish(topic, data string) { s.h.Publish(topic, sse.Event{Data: data}) }

// Core holds the shared dependencies and background routines. Fields are
// exported so tests may construct a partial Core literal directly.
type Core struct {
	Dir       string // config dir ($XDG_CONFIG_HOME/dunemgr or ~/.config/dunemgr)
	DataDir   string // data dir ($XDG_DATA_HOME/dunemgr or = Dir)
	BackupDir string // DataDir/backups (not created here; callers MkdirAll)

	Store    *store.Store
	SSH      *ssh.Client
	Hub      *sse.Hub
	Poller   *sse.Poller
	Schedule *schedule.Manager
}

// ConfigDir returns the dunemgr config directory: $XDG_CONFIG_HOME/dunemgr,
// or ~/.config/dunemgr when XDG_CONFIG_HOME is unset. getenv is injected for
// testability (pass os.Getenv in production).
func ConfigDir(getenv func(string) string) (string, error) {
	xdg := getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home: %w", err)
		}
		xdg = filepath.Join(home, ".config")
	}
	return filepath.Join(xdg, "dunemgr"), nil
}

// DataDir returns the dunemgr data directory: $XDG_DATA_HOME/dunemgr, or the
// config directory when XDG_DATA_HOME is unset.
func DataDir(getenv func(string) string) (string, error) {
	if xdg := getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "dunemgr"), nil
	}
	return ConfigDir(getenv)
}

// Open resolves the dirs, opens the bbolt store, and wires the SSH client,
// SSE hub, status poller, and schedule manager. The caller owns Close.
func Open(getenv func(string) string) (*Core, error) {
	dir, err := ConfigDir(getenv)
	if err != nil {
		return nil, err
	}
	data, err := DataDir(getenv)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(filepath.Join(data, "state.bolt"))
	if err != nil {
		return nil, err
	}
	sshClient := ssh.NewClient()
	hub := sse.NewHub()
	poller := &sse.Poller{
		Hub: hub,
		Hosts: func() ([]string, error) {
			profiles, err := st.ListHosts()
			if err != nil {
				return nil, err
			}
			names := make([]string, 0, len(profiles))
			for _, p := range profiles {
				names = append(names, p.Name)
			}
			return names, nil
		},
		Probe: func(ctx context.Context, host string) (store.StatusSnapshot, error) {
			return probe.Probe(ctx, st, sshClient, host)
		},
		Audit: func() ([]store.AuditEntry, error) { return st.ListAudit(0) },
	}
	mgr := schedule.NewManager(
		&broadcast.Runner{Exec: sshClient, Store: st},
		&lifecycle.Runner{SSH: sshClient, Store: st},
		st, ssePub{hub},
	)
	return &Core{
		Dir:       dir,
		DataDir:   data,
		BackupDir: filepath.Join(data, "backups"),
		Store:     st,
		SSH:       sshClient,
		Hub:       hub,
		Poller:    poller,
		Schedule:  mgr,
	}, nil
}

// Close releases the store (and thus the bbolt file lock).
func (c *Core) Close() error {
	if c.Store == nil {
		return nil
	}
	return c.Store.Close()
}
