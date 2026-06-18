package lifecycle

import (
	"context"
	"fmt"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Runner executes lifecycle actions and records audit entries.
type Runner struct {
	SSH   *ssh.Client
	Store *store.Store
	// Bin overrides DefaultBattlegroupBin. Empty = default.
	Bin string
}

// Result describes a successful lifecycle invocation.
type Result struct {
	Action Action
	Host   string
	Stdout string
	Stderr string
}

// Run executes one lifecycle verb on host. The action is forwarded
// to the Funcom vendor wrapper over SSH. An audit entry is appended
// after the call regardless of outcome.
func (r *Runner) Run(ctx context.Context, operator, host string, action Action) (*Result, error) {
	if !action.Valid() {
		return nil, fmt.Errorf("lifecycle Run: invalid action %q", action)
	}
	bin := r.Bin
	if bin == "" {
		bin = DefaultBattlegroupBin
	}
	res, err := r.SSH.Run(ctx, host, bin, string(action))

	entry := store.AuditEntry{
		Operator: operator,
		Host:     host,
		Action:   "lifecycle." + string(action),
		Subject:  fmt.Sprintf("host=%s", host),
	}
	switch {
	case err != nil:
		entry.Result = "error: " + err.Error()
		_ = r.Store.AppendAudit(entry)
		return nil, fmt.Errorf("lifecycle %s on %s: %w", action, host, err)
	case res.ExitCode != 0:
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		entry.Result = fmt.Sprintf("error: exit %d: %s", res.ExitCode, msg)
		_ = r.Store.AppendAudit(entry)
		return nil, fmt.Errorf("lifecycle %s on %s: exit %d: %s", action, host, res.ExitCode, msg)
	default:
		entry.Result = "ok"
		_ = r.Store.AppendAudit(entry)
	}
	return &Result{Action: action, Host: host, Stdout: res.Stdout, Stderr: res.Stderr}, nil
}
