// Package imageload loads the Funcom operator images (staged on the jumphost by
// the depot slice) into every schedulable node's containerd via a privileged
// import DaemonSet: it deploys the DaemonSet, then streams each tar into the
// per-node pods with `kubectl exec -i -- ctr images import -`. No in-cluster
// registry, no registries.yaml, no StorageClass, no node login — only the K8s API
// through the clusteraccess jumphost seam. See the images-load spec. rke2/k3s only
// (host containerd socket); Talos is a later seam.
//
// The DaemonSet carries no tolerations, so it lands on exactly the nodes that can
// run a pod — the same set the (also untolerating) operator Deployments schedule
// on. Hence every node that could host an operator has the image locally,
// regardless of whether the control plane is tainted (rke2 leaves it schedulable;
// kubeadm/k3s taint it).
package imageload

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"text/template"

	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

// Options configure the image load.
type Options struct {
	Namespace  string   // import DaemonSet namespace (default ds-arrakis-imageload)
	Tars       []string // absolute jumphost paths to the operator tars
	Socket     string   // host containerd socket (rke2: /run/k3s/containerd/containerd.sock)
	CtrPath    string   // host ctr binary (rke2: /var/lib/rancher/rke2/bin/ctr)
	KeepDaemon bool     // skip teardown (leave the DaemonSet for fast re-imports)
}

// Kubectl is the cluster-access seam: Run for read/apply-by-path/rollout/delete,
// Stdin to pipe bytes to a kubectl process (apply -f - and exec -i … import -).
type Kubectl interface {
	Run(ctx context.Context, args ...string) (string, error)
	Stdin(ctx context.Context, stdin []byte, args ...string) (string, error)
}

const dsName = "ds-arrakis-imageload"

// importerPods returns the import DaemonSet's pod names (one per schedulable node).
func importerPods(ctx context.Context, kc Kubectl, ns string) ([]string, error) {
	out, err := kc.Run(ctx, "get", "pods", "-n", ns, "-l", "app="+dsName,
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("list importer pods: %w", err)
	}
	pods := strings.Fields(out)
	if len(pods) == 0 {
		return nil, fmt.Errorf("no importer pods found in namespace %s", ns)
	}
	return pods, nil
}

// importTar streams one tar's bytes into pod's containerd via the host ctr,
// importing into the k8s.io namespace so the kubelet/IfNotPresent sees the image.
// The in-pod paths are fixed by the DaemonSet mounts (/host/bin/<ctr>,
// /host/containerd.sock); only the host ctr basename varies.
func importTar(ctx context.Context, kc Kubectl, ns, pod, ctrPath string, tar []byte) error {
	podCtr := "/host/bin/" + path.Base(ctrPath)
	if _, err := kc.Stdin(ctx, tar, "exec", "-i", pod, "-n", ns, "--",
		podCtr, "-a", "/host/containerd.sock", "-n", "k8s.io", "images", "import", "-"); err != nil {
		return fmt.Errorf("import into %s: %w", pod, err)
	}
	return nil
}

// Reader reads a staged tar off the jumphost (clusteraccess OnJump cat — binary
// safe: ssh.Result.Stdout is a verbatim byte-buffer cast).
type Reader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// Result reports what was loaded.
type Result struct {
	Pods []string // importer pods (one per schedulable node) the tars were streamed into
	Tars []string // tars imported
}

// Load deploys the import DaemonSet, then streams every operator tar into every
// importer pod's containerd. Idempotent: apply is declarative, ctr import
// overwrites, teardown is --ignore-not-found. Tears the DaemonSet down unless
// opts.KeepDaemon.
//
// ponytail: each tar is read to the workstation (OnJump cat) then streamed back
// to the jumphost-side kubectl (KubectlStdin) — a workstation<->jumphost
// round-trip per tar. Fine for the 4×~35MB operator tars on a one-shot load;
// reuses the existing seam with no new transport. Ceiling: a slow
// workstation->jumphost link; upgrade path = a jumphost-local `cat | kubectl
// exec` pipe (needs a new transport method, deliberately deferred).
func Load(ctx context.Context, kc Kubectl, r Reader, opts Options) (Result, error) {
	if len(opts.Tars) == 0 {
		return Result{}, fmt.Errorf("imageload: no operator tars found under the staging dir")
	}
	manifest, err := render(opts)
	if err != nil {
		return Result{}, fmt.Errorf("render import DaemonSet manifest: %w", err)
	}
	if _, err := kc.Stdin(ctx, manifest, "apply", "-f", "-"); err != nil {
		return Result{}, fmt.Errorf("apply import DaemonSet: %w", err)
	}
	if _, err := kc.Run(ctx, "-n", opts.Namespace, "rollout", "status",
		"daemonset/"+dsName, "--timeout=120s"); err != nil {
		return Result{}, fmt.Errorf("wait import DaemonSet rollout: %w", err)
	}
	pods, err := importerPods(ctx, kc, opts.Namespace)
	if err != nil {
		return Result{}, err
	}
	for _, tar := range opts.Tars {
		data, err := r.ReadFile(ctx, tar)
		if err != nil {
			return Result{}, fmt.Errorf("read tar %s: %w", tar, err)
		}
		for _, pod := range pods {
			if err := importTar(ctx, kc, opts.Namespace, pod, opts.CtrPath, data); err != nil {
				return Result{}, fmt.Errorf("tar %s: %w", tar, err)
			}
		}
	}
	if !opts.KeepDaemon {
		if _, err := kc.Run(ctx, "delete", "namespace", opts.Namespace, "--ignore-not-found"); err != nil {
			return Result{}, fmt.Errorf("teardown import DaemonSet: %w", err)
		}
	}
	return Result{Pods: pods, Tars: opts.Tars}, nil
}

// render produces the namespace + import DaemonSet manifest from opts.
func render(opts Options) ([]byte, error) {
	t, err := template.New("importer").Parse(importerTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse importer template: %w", err)
	}
	data := struct {
		Options
		CtrDir string
	}{opts, path.Dir(opts.CtrPath)}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render importer manifest: %w", err)
	}
	return buf.Bytes(), nil
}

// Jump runs a command on the jumphost (the tar host + ephemeral file server).
// Satisfied by *clusteraccess.Access.
type Jump interface {
	OnJump(ctx context.Context, name string, args ...string) (ssh.Result, error)
}

// HTTPOptions configures the HTTP-fetch import path for a large tar.
// Use this for tars that are too large for the kubectl-exec stream (~12 MB/s cap).
type HTTPOptions struct {
	Options
	TarPathOnJump string // absolute tar path on the jumphost (base name becomes the URL path)
	ServeDir      string // directory the jumphost file server roots at
	ServePort     int    // port the file server listens on
	JumpAddr      string // pod-reachable host:port for the importer pods to curl
}

// LoadViaHTTP deploys the import DaemonSet, starts an ephemeral static file
// server on the jumphost, has each importer pod curl the tar straight into
// `ctr images import -`, then stops the server. Used for the 4.2 GB BattleGroup
// runtime image where the kubectl-exec stream (~12 MB/s) is too slow.
//
// ponytail: the jumphost server is a nohup'd `python3 -m http.server` killed by
// its captured PID — a one-shot bring-up helper, not a long-lived service.
// Upgrade path: replace with a tiny purpose-built static server binary on the
// jumphost when python3 is absent or when concurrent downloads are needed.
func LoadViaHTTP(ctx context.Context, kc Kubectl, jp Jump, opts HTTPOptions) (Result, error) {
	manifest, err := render(opts.Options)
	if err != nil {
		return Result{}, fmt.Errorf("render import DaemonSet manifest: %w", err)
	}
	if _, err := kc.Stdin(ctx, manifest, "apply", "-f", "-"); err != nil {
		return Result{}, fmt.Errorf("apply import DaemonSet: %w", err)
	}
	if _, err := kc.Run(ctx, "-n", opts.Namespace, "rollout", "status",
		"daemonset/"+dsName, "--timeout=120s"); err != nil {
		return Result{}, fmt.Errorf("wait import DaemonSet rollout: %w", err)
	}
	pods, err := importerPods(ctx, kc, opts.Namespace)
	if err != nil {
		return Result{}, err
	}

	// Start an ephemeral HTTP file server on the jumphost; capture its PID for cleanup.
	pidRes, err := jp.OnJump(ctx, "sh", "-c", fmt.Sprintf(
		"cd %s && nohup python3 -m http.server %d >/dev/null 2>&1 & echo $!",
		opts.ServeDir, opts.ServePort))
	if err != nil {
		return Result{}, fmt.Errorf("start file server: %w", err)
	}
	pid := strings.TrimSpace(pidRes.Stdout)
	defer jp.OnJump(ctx, "kill", pid) //nolint:errcheck // best-effort teardown

	tarBasename := path.Base(opts.TarPathOnJump)
	url := fmt.Sprintf("http://%s/%s", opts.JumpAddr, tarBasename)
	podCtr := "/host/bin/" + path.Base(opts.CtrPath)
	curlCmd := fmt.Sprintf(
		"curl -fsSL %s | %s -a /host/containerd.sock -n k8s.io images import -",
		url, podCtr)

	for _, pod := range pods {
		if _, err := kc.Run(ctx, "exec", pod, "-n", opts.Namespace, "--",
			"sh", "-c", curlCmd); err != nil {
			return Result{}, fmt.Errorf("http import into %s: %w", pod, err)
		}
	}
	return Result{Pods: pods, Tars: []string{tarBasename}}, nil
}
