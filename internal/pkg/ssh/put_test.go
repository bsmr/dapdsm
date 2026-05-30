package ssh

import (
	"context"
	"testing"
)

func TestSendFileBuildsScpArgs(t *testing.T) {
	f := &fakeRunner{}
	c := &Client{Runner: f}
	err := c.SendFile(context.Background(), "vm-a", "/tmp/local", "/tmp/remote")
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if f.gotName != "scp" {
		t.Errorf("invoked %q, want scp", f.gotName)
	}
	want := []string{"-o", "BatchMode=yes", "--", "/tmp/local", "vm-a:/tmp/remote"}
	if len(f.gotArgs) != len(want) {
		t.Fatalf("args = %v, want %v", f.gotArgs, want)
	}
	for i := range want {
		if f.gotArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, f.gotArgs[i], want[i])
		}
	}
}

func TestSendFileRejectsFlagSmugglingHost(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.SendFile(context.Background(), "-oProxyCommand=evil", "/tmp/a", "/tmp/b")
	if err == nil {
		t.Errorf("SendFile with flag-prefixed host: want error, got nil")
	}
}

func TestSendFileRejectsFlagSmugglingLocalPath(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.SendFile(context.Background(), "vm-a", "-evil", "/tmp/b")
	if err == nil {
		t.Errorf("SendFile with flag-prefixed localPath: want error, got nil")
	}
}

func TestSendFileRejectsFlagSmugglingRemotePath(t *testing.T) {
	c := &Client{Runner: &fakeRunner{}}
	err := c.SendFile(context.Background(), "vm-a", "/tmp/a", "-evil")
	if err == nil {
		t.Errorf("SendFile with flag-prefixed remotePath: want error, got nil")
	}
}
