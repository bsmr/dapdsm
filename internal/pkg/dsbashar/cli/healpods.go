package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// podStatus is the subset of a Pod's status we care about when deciding
// whether to flag/delete it.
type podStatus struct {
	Name       string
	Phase      string
	Ready      bool
	AgeSeconds int
}

// kubeRunnerStub is the runner the listPods dependency receives.
// Production hands over the real kube.Runner; tests inject a noop value
// without dragging the full interface into the test helper signature.
type kubeRunnerStub = any

type healPodsDeps struct {
	runner   kube.Runner
	listPods func(ctx context.Context, runner kubeRunnerStub, namespace string) ([]podStatus, error)
}

func defaultHealPodsDeps(stderr io.Writer) healPodsDeps {
	r := &kube.CmdRunner{Stderr: stderr}
	return healPodsDeps{runner: r, listPods: kubectlListPods}
}

func healStuckPodsCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runHealStuckPods(ctx, args, stdout, stderr, defaultHealPodsDeps(stderr))
}

// runHealStuckPods walks the BG namespace looking for two failure
// patterns the Funcom-Operator does not auto-recover from:
//   - util-pods (`*-db-dbdepl-util-*`) in Phase=Failed/Error
//   - game-server pods (`*-sg-*`) older than --threshold-minutes but
//     not Ready
//
// Default is dry-run (report only). `--apply` force-deletes the
// flagged pods so the operator schedules fresh ones.
func runHealStuckPods(ctx context.Context, args []string, stdout, stderr io.Writer, deps healPodsDeps) error {
	fs := flag.NewFlagSet("heal-stuck-pods", flag.ContinueOnError)
	fs.SetOutput(stderr)
	ns := fs.String("namespace", "", "BattleGroup namespace (default: first funcom-seabass-*)")
	thresholdMin := fs.Int("threshold-minutes", 10, "Game-server pod age above which not-Ready counts as stuck")
	apply := fs.Bool("apply", false, "Actually delete the flagged pods (default: dry-run report)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: ds-bashar heal-stuck-pods [--apply] [--threshold-minutes N]\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(reorderFlagArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("heal-stuck-pods: %w: %w", ErrUsage, err)
	}
	if *ns == "" {
		found, err := kube.FindBattleGroupNamespace(ctx, deps.runner)
		if err != nil {
			return err
		}
		*ns = found
	}

	pods, err := deps.listPods(ctx, deps.runner, *ns)
	if err != nil {
		return err
	}

	var stuck []podStatus
	thresholdSec := *thresholdMin * 60
	for _, p := range pods {
		switch {
		case strings.Contains(p.Name, "-db-dbdepl-util-") &&
			(strings.EqualFold(p.Phase, "Failed") || strings.EqualFold(p.Phase, "Error")):
			stuck = append(stuck, p)
		case strings.Contains(p.Name, "-sg-") && !p.Ready && p.AgeSeconds >= thresholdSec:
			stuck = append(stuck, p)
		}
	}

	if len(stuck) == 0 {
		fmt.Fprintln(stdout, "heal-stuck-pods: no stuck pods found")
		return nil
	}
	for _, p := range stuck {
		if *apply {
			fmt.Fprintf(stdout, "heal-stuck-pods: delete %s (phase=%s ready=%t age=%ds)\n",
				p.Name, p.Phase, p.Ready, p.AgeSeconds)
			if _, err := deps.runner.Exec(ctx, *ns, "",
				"sh", "-c",
				fmt.Sprintf("kubectl delete pod %s -n %s --grace-period=0 --force",
					p.Name, *ns)); err != nil {
				fmt.Fprintf(stderr, "heal-stuck-pods: delete %s failed: %v\n", p.Name, err)
			}
		} else {
			fmt.Fprintf(stdout, "heal-stuck-pods: would delete %s (phase=%s ready=%t age=%ds) — dry-run\n",
				p.Name, p.Phase, p.Ready, p.AgeSeconds)
		}
	}
	return nil
}

// kubectlListPods is the production lister: calls `kubectl get pods -n NS
// -o json` and converts to []podStatus with ages in seconds against now().
func kubectlListPods(ctx context.Context, runner kubeRunnerStub, namespace string) ([]podStatus, error) {
	r, ok := runner.(kube.Runner)
	if !ok {
		return nil, fmt.Errorf("kubectlListPods: runner does not implement kube.Runner")
	}
	raw, err := r.Get(ctx, "pods", "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				StartTime         string `json:"startTime"`
				ContainerStatuses []struct {
					Ready bool `json:"ready"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode pod list: %w", err)
	}
	now := time.Now()
	out := make([]podStatus, 0, len(doc.Items))
	for _, it := range doc.Items {
		ready := false
		if len(it.Status.ContainerStatuses) > 0 {
			ready = it.Status.ContainerStatuses[0].Ready
		}
		var ageSec int
		if it.Status.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, it.Status.StartTime); err == nil {
				ageSec = int(now.Sub(t).Seconds())
			}
		}
		out = append(out, podStatus{
			Name:       it.Metadata.Name,
			Phase:      it.Status.Phase,
			Ready:      ready,
			AgeSeconds: ageSec,
		})
	}
	return out, nil
}
