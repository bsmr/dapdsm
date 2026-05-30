// Package server wires auth + ui into a single http.Handler.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/auth"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/backup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/broadcast"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/dbquery"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/hostpool"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/lifecycle"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/schedule"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/sse"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/store"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/ui"
	"go.muehmer.eu/dapdsm/internal/pkg/ssh"
)

// Options configure server.New.
type Options struct {
	Token string       // v1 bearer token
	Host  string       // canonical host:port for OriginGuard
	Store *store.Store // persistent state

	// Phase 3 additions — injectable for tests; nil => real defaults.
	LifecycleRunner *lifecycle.Runner // nil => constructed from SSHClient + Store
	BackupRunner    *backup.Runner    // nil => constructed from SSHClient + Store + BackupDataDir
	SSHClient       *ssh.Client       // nil => ssh.NewClient()
	BackupDataDir   string            // empty => XDG_DATA_HOME default (resolved at F2 use-site)

	// Phase 4 additions — injectable for tests; nil => real defaults.
	DBRunner *dbquery.Runner // nil => constructed from SSHClient + Store

	// Phase 5a addition — injectable for tests; nil => empty Hub.
	Hub *sse.Hub

	// Phase 5b addition — injectable for tests; nil => constructed from runners.
	ScheduleManager *schedule.Manager
}

// Server bundles the wired http.Handler plus its dependencies.
type Server struct {
	Handler http.Handler
	Auth    auth.Authenticator
	UI      *ui.Renderer
}

// New constructs the Foundation server: /login, /logout, / (home),
// /static/*.
func New(opts Options) (*Server, error) {
	r := ui.New()
	a := auth.NewTokenAuthenticator(opts.Token)

	mux := http.NewServeMux()

	// /login: GET renders template, POST delegates to auth handler.
	composedLogin := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			_ = r.Render(w, "login", nil)
			return
		}
		a.LoginHandler().ServeHTTP(w, req)
	})
	mux.Handle("/login", composedLogin)

	mux.Handle("GET /logout", a.LogoutHandler())
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(ui.StaticFS())))

	home := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		op, _ := a.Authenticate(req)
		_ = r.Render(w, "home", struct{ Operator auth.Operator }{op})
	})
	mux.Handle("GET /{$}", auth.RequireAuth(a)(home))

	notFound := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = r.RenderPartial(w, "notfound", map[string]any{"Path": req.URL.Path})
	})
	mux.Handle("/", auth.RequireAuth(a)(notFound))

	hosts := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		all, err := opts.Store.ListHosts()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type hostRow struct {
			Name    string
			BGState string
		}
		rows := make([]hostRow, 0, len(all))
		for _, h := range all {
			snap, _ := opts.Store.GetStatus(h.Name)
			rows = append(rows, hostRow{Name: h.Name, BGState: snap.BGState})
		}
		_ = r.RenderPartial(w, "sidebar", struct{ Hosts []hostRow }{rows})
	})
	mux.Handle("GET /hosts", auth.RequireAuth(a)(hosts))

	dashboard := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		name := req.PathValue("name")
		bg := req.URL.Query().Get("bg")
		if bg == "" {
			bg = name
		}
		snap, _ := opts.Store.GetStatus(name)
		entries, _ := opts.Store.ListAudit(0)
		var last *store.AuditEntry
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].Host == name {
				e := entries[i]
				last = &e
				break
			}
		}
		_ = r.Render(w, "dashboard", struct {
			Host       string
			BG         string
			Snap       store.StatusSnapshot
			LastAction *store.AuditEntry
		}{Host: name, BG: bg, Snap: snap, LastAction: last})
	})
	mux.Handle("GET /host/{name}", auth.RequireAuth(a)(dashboard))

	// Phase 3: lifecycle + backup runners (injectable for tests).
	sshClient := opts.SSHClient
	if sshClient == nil {
		sshClient = ssh.NewClient()
	}

	// hostAddView is the template data for the add-host form: the
	// submitted values (echoed back on error) plus an error message.
	type hostAddView struct {
		Name     string
		SSHAlias string
		Error    string
	}

	hostMgr := &hostpool.Manager{Store: opts.Store}

	hostAddForm := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = r.Render(w, "hostadd", hostAddView{})
	})
	mux.Handle("GET /hosts/add", auth.RequireAuth(a)(hostAddForm))

	hostAddSubmit := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(req.PostForm.Get("name"))
		alias := strings.TrimSpace(req.PostForm.Get("ssh_alias"))
		if alias == "" {
			alias = name
		}
		// Register fetches the K3s CA over SSH (blocking) — bound it.
		ctx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
		defer cancel()
		if err := hostMgr.Register(ctx, name, alias); err != nil {
			_ = r.Render(w, "hostadd", hostAddView{Name: name, SSHAlias: alias, Error: err.Error()})
			return
		}
		http.Redirect(w, req, "/host/"+name, http.StatusSeeOther)
	})
	mux.Handle("POST /hosts/add", auth.RequireAuth(a)(hostAddSubmit))

	lcRunner := opts.LifecycleRunner
	if lcRunner == nil {
		lcRunner = &lifecycle.Runner{SSH: sshClient, Store: opts.Store}
	}

	lifecycleHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		rawAction := req.PathValue("action")
		action, err := lifecycle.ValidateAction(rawAction)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		op, _ := a.Authenticate(req)
		res, runErr := lcRunner.Run(req.Context(), op.ID, host, action)
		if runErr != nil {
			http.Error(w, runErr.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "lifecycle %s ok\n%s", res.Action, res.Stdout)
	})
	mux.Handle("POST /host/{name}/lifecycle/{action}", auth.RequireAuth(a)(lifecycleHandler))

	lifecyclePartial := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		snap, err := opts.Store.GetStatus(host)
		found := err == nil
		_ = r.RenderPartial(w, "lifecycle", buildLifecycleView(host, snap, found, time.Now().UTC()))
	})
	mux.Handle("GET /host/{name}/lifecycle/_partial",
		auth.RequireAuth(a)(lifecyclePartial))

	bkRunner := opts.BackupRunner
	if bkRunner == nil {
		bkRunner = &backup.Runner{
			SSH:     sshClient,
			Store:   opts.Store,
			DataDir: opts.BackupDataDir,
		}
	}

	backupsList := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		bg := req.URL.Query().Get("bg")
		if bg == "" {
			http.Error(w, "bg= query parameter required", http.StatusBadRequest)
			return
		}
		rows, err := bkRunner.List(req.Context(), host, bg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = r.RenderPartial(w, "backups", map[string]any{
			"Host": host, "BG": bg, "Backups": rows,
		})
	})
	mux.Handle("GET /host/{name}/backups", auth.RequireAuth(a)(backupsList))

	backupsCreate := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		if err := req.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		bg := req.PostForm.Get("bg")
		name := req.PostForm.Get("name")
		if bg == "" || name == "" {
			http.Error(w, "bg and name required", http.StatusBadRequest)
			return
		}
		op, _ := a.Authenticate(req)
		rec, err := bkRunner.Create(req.Context(), op.ID, host, bg, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		rows, _ := bkRunner.List(req.Context(), host, bg)
		w.Header().Set("HX-Trigger", fmt.Sprintf("dunemgr-backup-created=%s", rec.Key()))
		_ = r.RenderPartial(w, "backups", map[string]any{
			"Host": host, "BG": bg, "Backups": rows,
		})
	})
	mux.Handle("POST /host/{name}/backups", auth.RequireAuth(a)(backupsCreate))

	backupsRestore := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		key := req.PostForm.Get("key")
		confirm := req.PostForm.Get("confirm")
		if key == "" || confirm != "yes" {
			http.Error(w, "key and confirm=yes required", http.StatusBadRequest)
			return
		}
		op, _ := a.Authenticate(req)
		if err := bkRunner.Restore(req.Context(), op.ID, key, true); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		fmt.Fprintln(w, "restore ok")
	})
	mux.Handle("POST /host/{name}/backups/restore", auth.RequireAuth(a)(backupsRestore))

	dbRunner := opts.DBRunner
	if dbRunner == nil {
		dbRunner = &dbquery.Runner{SSH: sshClient, Store: opts.Store}
	}

	dbSchema := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		tables, err := dbRunner.Tables(req.Context(), host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = r.RenderPartial(w, "db", map[string]any{
			"Host":   host,
			"Tables": tables,
		})
	})
	mux.Handle("GET /host/{name}/db", auth.RequireAuth(a)(dbSchema))

	dbExec := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		if err := req.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sql := strings.TrimSpace(req.PostForm.Get("sql"))
		if sql == "" {
			http.Error(w, "sql required", http.StatusBadRequest)
			return
		}
		if req.PostForm.Get("explain") == "1" {
			sql = "EXPLAIN (ANALYZE, FORMAT TEXT) " + sql
		}
		op, _ := a.Authenticate(req)
		res, err := dbRunner.Exec(req.Context(), op.ID, host, sql)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, res.Stdout)
		if res.Stderr != "" {
			fmt.Fprintln(w, "\n--- stderr ---")
			fmt.Fprint(w, res.Stderr)
		}
	})
	mux.Handle("POST /host/{name}/db/exec", auth.RequireAuth(a)(dbExec))

	dbColumns := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		schema := req.URL.Query().Get("schema")
		table := req.URL.Query().Get("table")
		cols, err := dbRunner.Columns(req.Context(), host, schema, table)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.RenderPartial(w, "db_columns", map[string]any{
			"Host": host, "Schema": schema, "Table": table, "Columns": cols,
		})
	})
	mux.Handle("GET /host/{name}/db/columns", auth.RequireAuth(a)(dbColumns))

	dbSlow := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		rows, err := dbRunner.SlowQueries(req.Context(), host, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = r.RenderPartial(w, "db_slow", map[string]any{"Host": host, "Rows": rows})
	})
	mux.Handle("GET /host/{name}/db/slow", auth.RequireAuth(a)(dbSlow))

	auditList := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		if limit <= 0 || limit > 500 {
			limit = 50
		}
		all, _ := opts.Store.ListAudit(0)
		// ListAudit is oldest-first; reverse for newest-first.
		reversed := make([]store.AuditEntry, len(all))
		for i, e := range all {
			reversed[len(all)-1-i] = e
		}
		if offset < 0 {
			offset = 0
		}
		if offset > len(reversed) {
			offset = len(reversed)
		}
		end := offset + limit
		if end > len(reversed) {
			end = len(reversed)
		}
		page := reversed[offset:end]
		_ = r.RenderPartial(w, "audit", map[string]any{
			"Entries": page,
			"Offset":  offset,
			"Limit":   limit,
		})
	})
	mux.Handle("GET /audit", auth.RequireAuth(a)(auditList))

	hub := opts.Hub
	if hub == nil {
		hub = sse.NewHub()
	}

	events := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		channel := req.PathValue("channel")
		switch channel {
		case "bg", "actions", "health":
		default:
			http.Error(w, "unknown channel", http.StatusNotFound)
			return
		}
		sse.ServeStream(w, req, hub, channel+"/"+host)
	})
	mux.Handle("GET /events/{name}/{channel}", auth.RequireAuth(a)(events))

	scheduleMgr := opts.ScheduleManager
	if scheduleMgr == nil {
		bcRunner := &broadcast.Runner{SSH: sshClient, Store: opts.Store}
		scheduleMgr = schedule.NewManager(bcRunner, lcRunner, opts.Store, hub)
	}

	shutdownSchedule := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		if err := req.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		action, err := lifecycle.ValidateAction(req.PostForm.Get("action"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		lead, _ := strconv.Atoi(req.PostForm.Get("lead"))
		kind := req.PostForm.Get("kind")
		if kind == "" {
			kind = "Restart"
		}
		op, _ := a.Authenticate(req)
		err = scheduleMgr.Schedule(req.Context(), op.ID, host, schedule.Request{
			Kind: kind, LeadSecs: lead, Action: action,
			ShutdownDurationS: 30, BroadcastFrequency: 60, BroadcastDuration: 10,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = r.RenderPartial(w, "shutdown", shutdownData(host, scheduleMgr))
	})
	mux.Handle("POST /host/{name}/shutdown", auth.RequireAuth(a)(shutdownSchedule))

	shutdownCancel := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		op, _ := a.Authenticate(req)
		if err := scheduleMgr.Cancel(req.Context(), op.ID, host); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		_ = r.RenderPartial(w, "shutdown", shutdownData(host, scheduleMgr))
	})
	mux.Handle("POST /host/{name}/shutdown/cancel", auth.RequireAuth(a)(shutdownCancel))

	shutdownPartial := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.PathValue("name")
		_ = r.RenderPartial(w, "shutdown", shutdownData(host, scheduleMgr))
	})
	mux.Handle("GET /host/{name}/shutdown/_partial", auth.RequireAuth(a)(shutdownPartial))

	// Origin guard wraps everything (it's a no-op on safe methods).
	guarded := auth.OriginGuard(opts.Host)(mux)

	return &Server{
		Handler: guarded,
		Auth:    a,
		UI:      r,
	}, nil
}

// shutdownData builds the template data for the shutdown partial:
// the host plus the pending countdown (if any).
func shutdownData(host string, mgr *schedule.Manager) map[string]any {
	rec, pending := mgr.Pending(host)
	return map[string]any{"Host": host, "Pending": pending, "Rec": rec}
}
