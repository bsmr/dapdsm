package ssh

import (
	"context"
	"testing"
)

func TestRecvFileBuildsScpArgs(t *testing.T) {
	rr := &recordingStdinRunner{}
	c := &Client{Runner: rr}
	if err := c.RecvFile(context.Background(), "vm-a", "/funcom/dump.backup", "/tmp/local.backup"); err != nil {
		t.Fatalf("RecvFile: %v", err)
	}
	got := joinArgs(rr.gotArgs)
	if rr.gotArgs[0] != "scp" {
		t.Errorf("argv[0]=%q, want scp", rr.gotArgs[0])
	}
	if !contains(got, "-o BatchMode=yes -o StrictHostKeyChecking=accept-new -- vm-a:/funcom/dump.backup /tmp/local.backup") {
		t.Errorf("argv tail mismatch: %q", got)
	}
}

func TestRecvFileRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &recordingStdinRunner{}}
	if err := c.RecvFile(context.Background(), "-evil", "/a", "/b"); err == nil {
		t.Error("RecvFile with flag-host err=nil, want non-nil")
	}
}

func TestRecvFileRejectsFlagSmugglingRemotePath(t *testing.T) {
	c := &Client{Runner: &recordingStdinRunner{}}
	if err := c.RecvFile(context.Background(), "vm-a", "-rf", "/b"); err == nil {
		t.Error("RecvFile with flag remotePath err=nil, want non-nil")
	}
}

func TestRecvFileRejectsFlagSmugglingLocalPath(t *testing.T) {
	c := &Client{Runner: &recordingStdinRunner{}}
	if err := c.RecvFile(context.Background(), "vm-a", "/r", "-rf"); err == nil {
		t.Error("RecvFile with flag localPath err=nil, want non-nil")
	}
}
