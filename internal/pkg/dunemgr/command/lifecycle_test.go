package command

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLifecycleUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := lifecycleCmd(context.Background(), nil, nil, &stdout, &stderr); err == nil {
		t.Error("lifecycle no args: err=nil, want non-nil")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Errorf("missing usage hint: %q", stderr.String())
	}
}

func TestLifecycleRejectsInvalidAction(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := lifecycleCmd(context.Background(), nil, []string{"vm-a", "destroy"}, &stdout, &stderr); err == nil {
		t.Error("lifecycle invalid action: err=nil, want non-nil")
	}
}
