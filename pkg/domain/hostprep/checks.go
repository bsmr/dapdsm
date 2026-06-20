package hostprep

import (
	"context"
	"fmt"
	"strings"
)

// Opts configures the dune operations user and the kubeconfig under check.
type Opts struct {
	User       string // operations user, default "dune"
	UID, GID   int    // desired UID/GID for creation (default 2000/2000)
	Kubeconfig string // kubeconfig path on the jumphost
}

func (o Opts) user() string {
	if o.User == "" {
		return "dune"
	}
	return o.User
}

func (o Opts) home() string { return "/home/" + o.user() }

// Check is one host-prep finding.
type Check struct {
	Name   string
	OK     bool
	Detail string
}

// HostChecks runs the Phase-B host-prep checks on the jumphost. Each check runs
// a command via the runner; OK means the command succeeded. Commands that need
// root use sudo internally (a no-op when already root).
func HostChecks(ctx context.Context, r Runner, o Opts) []Check {
	u := o.user()
	var checks []Check

	// privilege: root, or a passwordless sudoer.
	idRes, _ := r.OnJump(ctx, "id", "-u")
	root := strings.TrimSpace(idRes.Stdout) == "0"
	priv := Check{Name: "privilege", OK: root, Detail: "root"}
	if !root {
		if _, err := r.OnJump(ctx, "sudo", "-n", "true"); err == nil {
			priv.OK, priv.Detail = true, "passwordless sudoer"
		} else {
			priv.Detail = "neither root nor a passwordless sudoer"
		}
	}
	checks = append(checks, priv)

	// dune user exists (+ UID/GID).
	geRes, err := r.OnJump(ctx, "getent", "passwd", u)
	exists := err == nil && strings.HasPrefix(geRes.Stdout, u+":")
	uc := Check{Name: "user " + u + " exists", OK: exists}
	if exists {
		if f := strings.Split(strings.TrimSpace(geRes.Stdout), ":"); len(f) >= 4 {
			uc.Detail = fmt.Sprintf("uid=%s gid=%s", f[2], f[3])
		}
	} else {
		uc.Detail = "absent"
	}
	checks = append(checks, uc)

	// dune authorized_keys present (non-empty).
	_, akErr := r.OnJump(ctx, "sudo", "test", "-s", o.home()+"/.ssh/authorized_keys")
	checks = append(checks, Check{Name: u + " authorized_keys", OK: akErr == nil,
		Detail: o.home() + "/.ssh/authorized_keys"})

	// passwordless sudo drop-in for dune.
	_, sdErr := r.OnJump(ctx, "sudo", "test", "-f", "/etc/sudoers.d/"+u)
	checks = append(checks, Check{Name: u + " passwordless sudo", OK: sdErr == nil,
		Detail: "/etc/sudoers.d/" + u})

	// kubeconfig readable by dune.
	_, kcErr := r.OnJump(ctx, "sudo", "-u", u, "test", "-r", o.Kubeconfig)
	checks = append(checks, Check{Name: "kubeconfig readable by " + u, OK: kcErr == nil,
		Detail: o.Kubeconfig})

	return checks
}
