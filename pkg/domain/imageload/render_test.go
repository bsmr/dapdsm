package imageload

import (
	"strings"
	"testing"
)

func TestRenderDaemonSet(t *testing.T) {
	m, err := render(Options{
		Namespace: "ds-arrakis-imageload",
		Socket:    "/run/k3s/containerd/containerd.sock",
		CtrPath:   "/var/lib/rancher/rke2/bin/ctr",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(m)
	for _, want := range []string{
		"kind: Namespace",
		"name: ds-arrakis-imageload",
		"kind: DaemonSet",
		"app: ds-arrakis-imageload",
		"image: busybox:1.36",
		"privileged: true",
		"command: [\"sleep\", \"infinity\"]",
		"path: /run/k3s/containerd/containerd.sock", // host socket
		"type: Socket",
		"path: /var/lib/rancher/rke2/bin",           // host ctr dir (dirname of CtrPath)
		"mountPath: /host/containerd.sock",
		"mountPath: /host/bin",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("manifest missing %q\n---\n%s", want, s)
		}
	}
	// A DaemonSet with no tolerations must NOT carry one (so tainted control-plane
	// nodes are skipped and it lands on workers only).
	if strings.Contains(s, "tolerations:") {
		t.Errorf("manifest must not tolerate taints (lands on workers via CP taint)\n%s", s)
	}
}
