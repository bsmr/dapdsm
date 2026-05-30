package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/broadcast"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/config"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/lifecycle"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/probe"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/schedule"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/server"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// serveCmd starts the dunemgr web UI + background status poller and blocks
// until ctx is cancelled (SIGINT/SIGTERM, wired by main). It is the default
// action when dunemgr is invoked with no subcommand.
func serveCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	printToken := fs.Bool("print-token", false, "print the bearer token to stdout on start")
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir, err := configDir()
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	tok, err := config.EnsureToken(dir)
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

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
	scheduleMgr := schedule.NewManager(
		&broadcast.Runner{SSH: sshClient, Store: st},
		&lifecycle.Runner{SSH: sshClient, Store: st},
		st, hub,
	)
	srv, err := server.New(server.Options{
		Token:           tok,
		Host:            cfg.Bind,
		Store:           st,
		SSHClient:       sshClient,
		Hub:             hub,
		ScheduleManager: scheduleMgr,
	})
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr:              cfg.Bind,
		Handler:           srv.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(stdout, "dunemgr listening on http://%s/\n", cfg.Bind)
	if *printToken {
		fmt.Fprintf(stdout, "token: %s\n", tok)
	} else {
		fmt.Fprintf(stdout, "token stored in %s/token (read with: cat %s/token)\n", dir, dir)
	}

	go poller.Run(ctx)
	scheduleMgr.Rearm(ctx)
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
