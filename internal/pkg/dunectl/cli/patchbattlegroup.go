package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/internal/pkg/battlegroup"
	"go.muehmer.eu/dapdsm/internal/pkg/dunectl/config"
	"go.muehmer.eu/dapdsm/internal/pkg/kube"
	"go.muehmer.eu/dapdsm/internal/pkg/publicip"
)

// patchBgDeps groups the external dependencies of patchBattlegroup so
// tests can substitute fakes.
type patchBgDeps struct {
	runner   kube.Runner
	resolver publicip.Resolver
}

func defaultPatchBgDeps(stderr io.Writer) patchBgDeps {
	return patchBgDeps{
		runner:   &kube.CmdRunner{Stderr: stderr},
		resolver: &publicip.HTTPResolver{},
	}
}

func patchBattlegroupCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return patchBattlegroup(ctx, args, stdout, stderr, defaultPatchBgDeps(stderr))
}

func patchBattlegroup(ctx context.Context, args []string, stdout, stderr io.Writer, deps patchBgDeps) error {
	fs := flag.NewFlagSet("patch-battlegroup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ip := fs.String("ip", "", "Player-facing IPv4 to set in the CR (default: K3s node ExternalIP, then api.ipify.org fallback)")
	id := fs.String("id", "", "Short HOST_DATACENTER_ID identifier (default: from dunectl.env HOST_DATACENTER_ID, empty=leave Funcom default in place)")
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-* namespace)")
	bg := fs.String("bg-name", "", "BattleGroup name (default: derived from --namespace)")
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("patch-battlegroup: %w: %w", ErrUsage, err)
	}

	if *ns == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, deps.runner)
		if err != nil {
			return err
		}
		*ns = found
	}
	if *bg == "" {
		*bg = kube.BattleGroupName(*ns)
	}
	if *ip == "" {
		// Prefer the K3s node ExternalIP: on multi-public-IP setups the
		// outbound NAT pool returns a different address than the one
		// assigned to this VM, so api.ipify.org would resolve to the
		// neighbour's IP. The node ExternalIP is set deliberately via
		// K3S_NODE_EXTERNAL_IP in etc/k3s/install.sh.
		if nodeIP, err := kube.NodeExternalIP(ctx, deps.runner); err == nil {
			*ip = nodeIP
		} else {
			fmt.Fprintf(stderr, "patch-battlegroup: K3s node ExternalIP unavailable (%v); falling back to egress lookup\n", err)
			resolved, rerr := deps.resolver.Resolve(ctx)
			if rerr != nil {
				return fmt.Errorf("resolve public IPv4: %w", rerr)
			}
			*ip = resolved
		}
	}
	// HOST_DATACENTER_ID is opt-in: --id wins, otherwise read from
	// /etc/dune/dunectl.env (HOST_DATACENTER_ID=…). Empty means leave the
	// Funcom-template default ("dune-testing") untouched.
	if *id == "" {
		if cfg, err := config.LoadFromFile(config.DefaultPath); err == nil {
			*id = cfg.HostDatacenterID
		}
	}

	fmt.Fprintf(stdout, "namespace:   %s\n", *ns)
	fmt.Fprintf(stdout, "battlegroup: %s\n", *bg)
	fmt.Fprintf(stdout, "player IP:   %s\n", *ip)
	if *id != "" {
		fmt.Fprintf(stdout, "host id:     %s\n", *id)
	}

	cr, err := deps.runner.Get(ctx, "battlegroup", *bg, "-n", *ns, "-o", "json")
	if err != nil {
		return err
	}

	ipOps, err := battlegroup.BuildHostIPPatches(cr, *ip)
	if err != nil {
		return err
	}
	idOps, err := battlegroup.BuildHostIDPatches(cr, *id)
	if err != nil {
		return err
	}
	schedOps, err := battlegroup.BuildSchedulerNameRemovals(cr)
	if err != nil {
		return err
	}

	for _, ops := range [][]battlegroup.Operation{ipOps, idOps, schedOps} {
		if len(ops) == 0 {
			continue
		}
		payload, err := json.Marshal(ops)
		if err != nil {
			return err
		}
		if err := deps.runner.Patch(ctx, "battlegroup", *bg, *ns, "json", string(payload)); err != nil {
			return err
		}
	}

	fmt.Fprintf(stdout, "applied %d host-IP op(s), %d host-ID op(s), and %d scheduler-removal op(s)\n",
		len(ipOps), len(idOps), len(schedOps))
	return nil
}
