package operatorbringup

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeKubectl records Run argv and Apply manifests; get-secret returns notFound
// so the webhook-secret guard takes the apply path.
//
// calls is a unified, ordered log of every interaction:
//   - Run appends "run:<joined args>" (space-joined argv).
//   - Apply appends "apply-stdin:<first line of manifest>" so ordering
//     assertions can distinguish the operator manifest from webhook secrets.
type fakeKubectl struct {
	runs    [][]string
	applied []string
	calls   []string // unified ordered log: "run:<args>" | "apply-stdin:<first-line>"
}

func (f *fakeKubectl) Run(_ context.Context, args ...string) (string, error) {
	f.runs = append(f.runs, args)
	f.calls = append(f.calls, "run:"+strings.Join(args, " "))
	// Return worker nodes for kubectl get nodes queries.
	if len(args) >= 2 && args[0] == "get" && args[1] == "nodes" {
		return "w1 w2", nil
	}
	// Simulate a fresh cluster: any other "get <resource> ..." call returns notFound so
	// that existence guards (cert-manager deployment, webhook secrets) all take
	// their apply path.
	if len(args) >= 1 && args[0] == "get" {
		return "", errors.New("not found")
	}
	return "", nil
}
func (f *fakeKubectl) Apply(_ context.Context, manifest []byte) (string, error) {
	f.applied = append(f.applied, string(manifest))
	// Build a synthetic log tag that uniquely identifies the manifest:
	// use the first substring from a priority list that appears in the content.
	// This lets ordering assertions distinguish the operator manifest from
	// webhook secrets or namespace manifests without embedding full content.
	tag := "unknown"
	s := string(manifest)
	switch {
	case strings.Contains(s, "battlegroupoperator-controller-manager"):
		tag = "operator-deployments"
	case strings.Contains(s, "kubernetes.io/tls"):
		tag = "webhook-secret"
	case strings.Contains(s, "kind: Namespace"):
		tag = "namespace"
	}
	f.calls = append(f.calls, "apply-stdin:"+tag)
	return "", nil
}

func runIndex(runs [][]string, sub string) int {
	for i, r := range runs {
		if strings.Contains(strings.Join(r, " "), sub) {
			return i
		}
	}
	return -1
}

func TestBringUp_OrdersAndCovers(t *testing.T) {
	f := &fakeKubectl{}
	opts := Options{
		Version:        "v1.5.0",
		CRDDir:         "/home/dune/depot/prod/images/operators/crds",
		CertManagerURL: "https://example/cert-manager.yaml",
	}
	if err := BringUp(context.Background(), f, opts); err != nil {
		t.Fatalf("BringUp: %v", err)
	}
	joinedRuns := func() string {
		var b strings.Builder
		for _, r := range f.runs {
			b.WriteString(strings.Join(r, " ") + "\n")
		}
		return b.String()
	}()
	// cert-manager applied (url non-empty): the URL appears in a Run("apply",...,url) call.
	if !strings.Contains(joinedRuns, "cert-manager.yaml") {
		t.Error("cert-manager url not applied")
	}
	// workers discovered via kubectl get nodes and labeled
	for _, w := range []string{"w1", "w2"} {
		if !strings.Contains(joinedRuns, "label node "+w+" node.funcom.com/workload=infrastructure --overwrite") {
			t.Errorf("worker %s not labeled", w)
		}
	}
	// CRDs applied server-side from the jumphost dir
	if !strings.Contains(joinedRuns, "apply --server-side --validate=false -f "+opts.CRDDir) {
		t.Errorf("CRDs not applied server-side:\n%s", joinedRuns)
	}
	// 4 webhook secrets + the operator manifest applied via Apply (stdin)
	secretCount := 0
	operatorManifestApplied := false
	for _, m := range f.applied {
		if strings.Contains(m, "kubernetes.io/tls") {
			secretCount++
		}
		if strings.Contains(m, "battlegroupoperator-controller-manager") {
			operatorManifestApplied = true
		}
	}
	if secretCount != 4 {
		t.Errorf("want 4 webhook secrets applied, got %d", secretCount)
	}
	if !operatorManifestApplied {
		t.Error("operator manifest not applied")
	}
	// CRDs must be applied before the operator deployments are waited on.
	// Use the namespace-scoped wait to avoid matching the cert-manager wait.
	crdIdx := runIndex(f.runs, "apply --server-side --validate=false -f "+opts.CRDDir)
	waitIdx := runIndex(f.runs, "wait --for=condition=Available -n "+namespace)
	if crdIdx == -1 || waitIdx == -1 || crdIdx > waitIdx {
		t.Errorf("ordering wrong: crdIdx=%d waitIdx=%d", crdIdx, waitIdx)
	}
	// Cross-check via the unified call log: the CRD apply (Run) must appear
	// before the operator-manifest apply (Apply via stdin, tagged "operator-deployments").
	// This catches a reorder where Apply(operatorManifest) is moved before
	// Run("apply","--server-side",...) — something the runs/applied split log cannot catch.
	unifCRDIdx := -1
	unifOpIdx := -1
	for i, entry := range f.calls {
		if unifCRDIdx == -1 && strings.Contains(entry, "apply --server-side --validate=false -f "+opts.CRDDir) {
			unifCRDIdx = i
		}
		if unifOpIdx == -1 && entry == "apply-stdin:operator-deployments" {
			unifOpIdx = i
		}
	}
	if unifCRDIdx == -1 || unifOpIdx == -1 || unifCRDIdx > unifOpIdx {
		t.Errorf("unified-log ordering wrong: CRD apply at %d, operator-manifest apply at %d (calls: %v)",
			unifCRDIdx, unifOpIdx, f.calls)
	}
	// each operator waited Available
	for _, op := range []string{"battlegroupoperator", "databaseoperator", "serveroperator", "utilitiesoperator"} {
		if !strings.Contains(joinedRuns, "deployment/"+op+"-controller-manager") {
			t.Errorf("op %s not waited", op)
		}
	}
	// get nodes call must appear between namespace apply and CRD apply (ordering check)
	getNodesCallIdx := -1
	nsApplyIdx := -1
	for i, c := range f.calls {
		if nsApplyIdx == -1 && c == "apply-stdin:namespace" {
			nsApplyIdx = i
		}
		if getNodesCallIdx == -1 && strings.Contains(c, "get nodes") {
			getNodesCallIdx = i
		}
	}
	if nsApplyIdx == -1 || getNodesCallIdx == -1 {
		t.Errorf("namespace apply or get nodes not found in calls: nsApplyIdx=%d getNodesCallIdx=%d",
			nsApplyIdx, getNodesCallIdx)
	}
	if nsApplyIdx > getNodesCallIdx {
		t.Errorf("namespace apply must come before get nodes: nsApplyIdx=%d getNodesCallIdx=%d",
			nsApplyIdx, getNodesCallIdx)
	}
	if getNodesCallIdx > unifCRDIdx {
		t.Errorf("get nodes must come before CRD apply: getNodesCallIdx=%d unifCRDIdx=%d",
			getNodesCallIdx, unifCRDIdx)
	}
}

func TestBringUp_SkipsCertManagerWhenURLEmpty(t *testing.T) {
	f := &fakeKubectl{}
	opts := Options{
		Version: "v1", CRDDir: "/d/crds",
		CertManagerURL: "",
	}
	if err := BringUp(context.Background(), f, opts); err != nil {
		t.Fatalf("BringUp: %v", err)
	}
	for _, r := range f.runs {
		if strings.Contains(strings.Join(r, " "), "cert-manager") {
			t.Errorf("cert-manager touched despite empty URL: %v", r)
		}
	}
}

func TestWorkerNodes(t *testing.T) {
	t.Run("returns worker names", func(t *testing.T) {
		f := &fakeKubectl{}
		got, err := workerNodes(context.Background(), f)
		if err != nil {
			t.Fatalf("workerNodes: %v", err)
		}
		// fakeKubectl returns "w1 w2" for get nodes calls
		if len(got) != 2 || got[0] != "w1" || got[1] != "w2" {
			t.Errorf("unexpected nodes: %v", got)
		}
		// assert argv contains expected args
		if len(f.runs) == 0 {
			t.Fatal("no runs recorded")
		}
		joined := strings.Join(f.runs[0], " ")
		if !strings.Contains(joined, "get nodes") {
			t.Errorf("expected get nodes in args, got: %s", joined)
		}
		if !strings.Contains(joined, "!node-role.kubernetes.io/control-plane") {
			t.Errorf("expected label selector in args, got: %s", joined)
		}
		if !strings.Contains(joined, "jsonpath={.items[*].metadata.name}") {
			t.Errorf("expected jsonpath in args, got: %s", joined)
		}
	})

	t.Run("errors on empty output", func(t *testing.T) {
		// fakeKubectl that returns empty for get nodes
		emptyKC := &emptyNodesKubectl{}
		_, err := workerNodes(context.Background(), emptyKC)
		if err == nil {
			t.Fatal("expected error when no worker nodes found")
		}
		if !strings.Contains(err.Error(), "no worker nodes found") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// emptyNodesKubectl returns empty string for get nodes calls (simulates
// a cluster where all nodes are control-plane).
type emptyNodesKubectl struct{}

func (e *emptyNodesKubectl) Run(_ context.Context, args ...string) (string, error) {
	if len(args) >= 2 && args[0] == "get" && args[1] == "nodes" {
		return "", nil
	}
	if len(args) >= 1 && args[0] == "get" {
		return "", errors.New("not found")
	}
	return "", nil
}
func (e *emptyNodesKubectl) Apply(_ context.Context, _ []byte) (string, error) {
	return "", nil
}
