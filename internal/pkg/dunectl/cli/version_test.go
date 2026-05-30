package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestVersion_PrintsToStdout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if err := versionCmd(context.Background(), nil, &stdout, &stderr); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "dunectl ") {
		t.Errorf("stdout = %q, want it to start with 'dunectl '", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}
