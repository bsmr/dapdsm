package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/config"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/core"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/server"
)

// serveCmd starts the dunemgr web UI + background status poller and blocks
// until ctx is cancelled. Default action when dunemgr runs with no subcommand.
func serveCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	printToken := fs.Bool("print-token", false, "print the bearer token to stdout on start")
	if err := fs.Parse(args); err != nil {
		return err
	}

	c, err := core.Open(os.Getenv)
	if err != nil {
		return err
	}
	defer c.Close()

	cfg, err := config.Load(c.Dir)
	if err != nil {
		return err
	}
	tok, err := config.EnsureToken(c.Dir)
	if err != nil {
		return err
	}

	srv, err := server.New(server.Options{
		Token:           tok,
		Host:            cfg.Bind,
		Store:           c.Store,
		SSHClient:       c.SSH,
		Hub:             c.Hub,
		ScheduleManager: c.Schedule,
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
		fmt.Fprintf(stdout, "token stored in %s/token (read with: cat %s/token)\n", c.Dir, c.Dir)
	}

	go c.Poller.Run(ctx)
	c.Schedule.Rearm(ctx)
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
