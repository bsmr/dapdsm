package hostprep

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// Step is one remediation command run on the jumphost via Runner.OnJump.
// Argv[0] is the command, Argv[1:] its arguments.
type Step struct {
	Desc string
	Argv []string
}

// PreparePlan builds the ordered, idempotent remediation for the dune user.
// Every command guards itself (no-op if already satisfied) and uses sudo
// internally, so it is safe to run as root or as a passwordless sudoer.
func PreparePlan(o Opts, migrate []string) []Step {
	u := o.user()
	uid, gid := o.UID, o.GID
	if uid == 0 {
		uid = 2000
	}
	if gid == 0 {
		gid = 2000
	}
	home := "/home/" + u

	userScript := fmt.Sprintf(
		"getent group %s >/dev/null || groupadd -g %d %s; "+
			"id -u %s >/dev/null 2>&1 || useradd -m -u %d -g %d -s /bin/bash %s",
		u, gid, u, u, uid, gid, u)

	// $HOME is the invoking user's home (the outer sh is NOT under sudo); the
	// inner sudo writes into dune's home as root.
	keysScript := fmt.Sprintf(
		"sudo install -d -o %s -g %s -m 700 %s/.ssh && "+
			"sudo install -o %s -g %s -m 600 \"$HOME/.ssh/authorized_keys\" %s/.ssh/authorized_keys",
		u, u, home, u, u, home)

	sudoersScript := fmt.Sprintf(
		"printf '%%s\\n' '%s ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/%s && "+
			"chmod 0440 /etc/sudoers.d/%s && visudo -cf /etc/sudoers.d/%s",
		u, u, u, u)

	steps := []Step{
		{Desc: "create group+user " + u, Argv: []string{"sudo", "sh", "-c", userScript}},
		{Desc: "copy authorized_keys to " + u, Argv: []string{"sh", "-c", keysScript}},
		{Desc: "passwordless sudo for " + u, Argv: []string{"sudo", "sh", "-c", sudoersScript}},
	}

	for _, src := range migrate {
		mig := fmt.Sprintf("sudo install -D -o %s -g %s -m 600 %q %s/%s",
			u, u, src, home, baseName(src))
		steps = append(steps, Step{
			Desc: "migrate config " + src + " -> " + home, Argv: []string{"sh", "-c", mig}})
	}
	return steps
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// Apply runs each remediation step on the jumphost, stopping on the first error.
// dryRun prints the steps without executing them.
func Apply(ctx context.Context, r Runner, steps []Step, dryRun bool, out io.Writer) error {
	for _, s := range steps {
		if dryRun {
			fmt.Fprintf(out, "DRY-RUN: %s\n  $ %s\n", s.Desc, strings.Join(s.Argv, " "))
			continue
		}
		fmt.Fprintf(out, "%s\n", s.Desc)
		if _, err := r.OnJump(ctx, s.Argv[0], s.Argv[1:]...); err != nil {
			return fmt.Errorf("step %q: %w", s.Desc, err)
		}
	}
	return nil
}
