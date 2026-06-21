package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/domain/hostprep"
	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// ErrGateFailed is returned when the Phase-A cluster-access gate does not pass.
var ErrGateFailed = errors.New("cluster-access gate failed")

// realRunner builds the production hostprep.Runner: a clusteraccess.Access over
// the system ssh client, with a minimal descriptor (jumphost + kubeconfig; no
// inventory — doctor/prepare-host do not need the node list).
func realRunner(jump, kube string) hostprep.Runner {
	return clusteraccess.New(ssh.NewClient(), &clusteraccess.Descriptor{JumpHost: jump, Kubeconfig: kube})
}

// doctorCmd diagnoses the jumphost: Phase A cluster-access gate (hard stop on
// failure), then Phase B host-prep checks. The runner is injected for tests; the
// dispatcher wires the real clusteraccess.Access.
func doctorCmd(ctx context.Context, newRunner func(jump, kube string) hostprep.Runner, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jump := fs.String("jump", "", "jumphost ssh-config alias")
	kubeconfig := fs.String("kubeconfig", "", "kubeconfig path on the jumphost")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jump == "" || *kubeconfig == "" {
		fmt.Fprintln(stderr, "doctor: --jump and --kubeconfig are required")
		return ErrUsage
	}
	r := newRunner(*jump, *kubeconfig)

	fmt.Fprintln(stdout, "== cluster-access gate ==")
	gate := hostprep.ClusterGate(ctx, r)
	for _, p := range gate.Probes {
		fmt.Fprintf(stdout, "  %s %s%s\n", mark(p.OK), p.Name, detail(p.Detail))
	}
	if !gate.Pass {
		fmt.Fprintln(stderr, "doctor: cluster-access gate failed — use an admin/cluster-scoped kubeconfig")
		return ErrGateFailed
	}

	fmt.Fprintln(stdout, "== host-prep checks ==")
	for _, c := range hostprep.HostChecks(ctx, r, hostprep.Opts{User: "dune", Kubeconfig: *kubeconfig}) {
		fmt.Fprintf(stdout, "  %s %s%s\n", mark(c.OK), c.Name, detail(c.Detail))
	}

	fmt.Fprintln(stdout, "== cluster scheduling ==")
	cp := hostprep.ControlPlaneTaint(ctx, r)
	fmt.Fprintf(stdout, "  %s %s%s\n", mark(cp.OK), cp.Name, detail(cp.Detail))

	return nil
}

func mark(ok bool) string {
	if ok {
		return "[OK]"
	}
	return "[!!]"
}

func detail(d string) string {
	if d == "" {
		return ""
	}
	return " (" + d + ")"
}
