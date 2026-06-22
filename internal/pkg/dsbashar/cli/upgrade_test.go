package cli

import (
	"bytes"
	"context"
	"io"
	"testing"

	"go.muehmer.eu/dapdsm/pkg/transport/clusteraccess"
	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

func TestUpgradeRequiresJump(t *testing.T) {
	prev := resolvedAccess
	defer func() { resolvedAccess = prev }()
	resolvedAccess = nil // single-node: upgrade is multi-node only

	var out, errb bytes.Buffer
	err := upgradeCmd(context.Background(), []string{"--image-tag", "9-0-shipping"}, &out, &errb)
	if err == nil {
		t.Fatal("want ErrUsage without --jump, got nil")
	}
}

func TestUpgradeRequiresImageTag(t *testing.T) {
	prevA, prevR := resolvedAccess, kubeRunnerFor
	defer func() { resolvedAccess = prevA; kubeRunnerFor = prevR }()
	resolvedAccess = &clusteraccess.Access{}
	kubeRunnerFor = func(io.Writer) kube.Runner { return &fakeKubeRunner{ns: "funcom-seabass-x"} }

	var out, errb bytes.Buffer
	if err := upgradeCmd(context.Background(), nil, &out, &errb); err == nil {
		t.Fatal("want ErrUsage without --image-tag, got nil")
	}
}
