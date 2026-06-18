package backup

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"go.muehmer.eu/dapdsm/pkg/domain/store"
	"go.muehmer.eu/dapdsm/pkg/transport/ssh"
)

func TestListReturnsBoundStoreRows(t *testing.T) {
	st := newStore(t)
	now := time.Now().Unix()
	for _, ts := range []int64{now, now - 100} {
		_ = st.PutBackup(store.BackupRecord{
			Host: "vm-a", BG: "bg", Name: "n", UnixTS: ts,
			LocalPath: filepath.Join(t.TempDir(), "x.backup"),
			CreatedAt: time.Unix(ts, 0).UTC(),
		})
	}
	r := &Runner{SSH: &ssh.Client{Runner: &sshFake{}}, Store: st, DataDir: t.TempDir()}
	rows, err := r.List(context.Background(), "vm-a", "bg")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("len=%d, want 2", len(rows))
	}
	if rows[0].UnixTS < rows[1].UnixTS {
		t.Errorf("not newest-first: %+v", rows)
	}
}
