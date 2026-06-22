package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"go.muehmer.eu/dapdsm/pkg/domain/battlegroup"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

type updateDeps struct {
	runVendor    vendorRunner
	announceDeps announceDeps
}

func defaultUpdateDeps() updateDeps {
	return updateDeps{runVendor: execVendor, announceDeps: defaultAnnounceDeps()}
}

func updateCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runUpdate(ctx, args, stdout, stderr, defaultUpdateDeps())
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer, deps updateDeps) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bin := fs.String("bg-binary", DefaultBattlegroupBin, "Path to Funcom's battlegroup wrapper (single-node fallback)")
	noRestart := fs.Bool("no-restart", false, "Skip the final 'battlegroup restart' (single-node)")
	fromDownloads := fs.Bool("from-downloads", false, "Use 'update-from-downloads' (single-node)")
	announce := fs.Duration("announce", 0, "Announce a shutdown countdown of this duration, then act (e.g. 5m)")
	ns := fs.String("namespace", "", "BattleGroup namespace (K8s-native path)")
	bg := fs.String("bg-name", "", "BattleGroup name (K8s-native path)")
	imageTag := fs.String("image-tag", "", "Depot revision tag to reconcile onto the CR (K8s-native path; required when --jump set)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("update: %w: %w", ErrUsage, err)
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "update: unexpected positional argument(s): %v\n", fs.Args())
		return ErrUsage
	}

	if resolvedAccess != nil {
		if *imageTag == "" {
			fmt.Fprintln(stderr, "update: --image-tag is required on the K8s-native path")
			return ErrUsage
		}
		return withAnnounce(ctx, *announce, "Update", func(ctx context.Context) error {
			r := newKubeRunner(stderr)
			n, b := *ns, *bg
			if n == "" {
				found, err := kube.FindBattleGroupNamespace(ctx, r)
				if err != nil {
					return err
				}
				n = found
			}
			if b == "" {
				b = kube.BattleGroupName(n)
			}
			return battlegroup.Update(ctx, r, n, b, *imageTag)
		}, deps.announceDeps)
	}

	updateAction := "update"
	if *fromDownloads {
		updateAction = "update-from-downloads"
	}
	return withAnnounce(ctx, *announce, "Update", func(ctx context.Context) error {
		if err := deps.runVendor(ctx, *bin, updateAction, stdout, stderr); err != nil {
			return err
		}
		if err := deps.runVendor(ctx, *bin, "apply-default-usersettings", stdout, stderr); err != nil {
			return err
		}
		if !*noRestart {
			if err := deps.runVendor(ctx, *bin, "restart", stdout, stderr); err != nil {
				return err
			}
		}
		return nil
	}, deps.announceDeps)
}
