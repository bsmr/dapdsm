package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.muehmer.eu/dapdsm/internal/pkg/dsbashar/config"
)

type stepRecorder struct {
	calls []string
}

func (s *stepRecorder) initDB(_ context.Context, _, _ io.Writer) error {
	s.calls = append(s.calls, "init-db")
	return nil
}
func (s *stepRecorder) patchBg(_ context.Context, _, _ io.Writer) error {
	s.calls = append(s.calls, "patch-battlegroup")
	return nil
}
func (s *stepRecorder) patchPorts(_ context.Context, gameBase, igwBase int, _, _ io.Writer) error {
	s.calls = append(s.calls, "patch-game-ports")
	return nil
}
func (s *stepRecorder) enableSet(_ context.Context, m string, _, _ io.Writer) error {
	s.calls = append(s.calls, "enable-set:"+m)
	return nil
}
func (s *stepRecorder) iniSet(_ context.Context, key, _ string, applyRestart bool, _, _ io.Writer) error {
	tag := "ini-set:" + key
	if applyRestart {
		tag += "+apply+restart"
	}
	s.calls = append(s.calls, tag)
	return nil
}

func TestReconcile_RunsMinimalSequenceWhenOnlyTargetSet(t *testing.T) {
	t.Parallel()
	rec := &stepRecorder{}
	deps := reconcileDeps{
		cfg:        config.Config{Target: config.TargetProd},
		initDB:     rec.initDB,
		patchBg:    rec.patchBg,
		patchPorts: rec.patchPorts,
		enableSet:  rec.enableSet,
		iniSet:     rec.iniSet,
	}
	var stdout, stderr bytes.Buffer
	if err := runReconcile(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"init-db", "patch-battlegroup"}
	if !slices.Equal(rec.calls, want) {
		t.Errorf("calls = %v, want %v", rec.calls, want)
	}
}

func TestReconcile_RunsAllStepsWhenFullCfgGiven(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pwFile := filepath.Join(tmp, "server-pw")
	if err := os.WriteFile(pwFile, []byte("server-secret"), 0o600); err != nil {
		t.Fatalf("write pw: %v", err)
	}
	rec := &stepRecorder{}
	deps := reconcileDeps{
		cfg: config.Config{
			Target:             config.TargetProd,
			GamePortBase:       7877,
			IGWPortBase:        7988,
			AlwaysOnSets:       []string{"SH_Arrakeen", "DeepDesert_1"},
			ServerDisplayName:  "Offworld",
			ServerPasswordFile: pwFile,
		},
		initDB:     rec.initDB,
		patchBg:    rec.patchBg,
		patchPorts: rec.patchPorts,
		enableSet:  rec.enableSet,
		iniSet:     rec.iniSet,
	}
	var stdout, stderr bytes.Buffer
	if err := runReconcile(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{
		"init-db",
		"patch-battlegroup",
		"patch-game-ports",
		"enable-set:SH_Arrakeen",
		"enable-set:DeepDesert_1",
		"ini-set:Bgd.ServerDisplayName",
		"ini-set:Bgd.ServerLoginPassword+apply+restart",
	}
	if !slices.Equal(rec.calls, want) {
		t.Errorf("calls =\n  got  %v\n  want %v", rec.calls, want)
	}
}

func TestReconcile_HonoursSkipInitDB(t *testing.T) {
	t.Parallel()
	rec := &stepRecorder{}
	deps := reconcileDeps{
		cfg:        config.Config{Target: config.TargetProd, SkipInitDB: true},
		initDB:     rec.initDB,
		patchBg:    rec.patchBg,
		patchPorts: rec.patchPorts,
		enableSet:  rec.enableSet,
		iniSet:     rec.iniSet,
	}
	var stdout, stderr bytes.Buffer
	if err := runReconcile(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(rec.calls, "init-db") {
		t.Errorf("init-db should be skipped; got %v", rec.calls)
	}
}

func TestReconcile_SkipsPortPatchWhenOnlyOneBaseGiven(t *testing.T) {
	t.Parallel()
	// Asymmetric configuration: only game-base, no igw-base → safer to
	// skip rather than emit a half-shifted port patch.
	rec := &stepRecorder{}
	deps := reconcileDeps{
		cfg: config.Config{
			Target:       config.TargetProd,
			GamePortBase: 7877,
		},
		initDB:     rec.initDB,
		patchBg:    rec.patchBg,
		patchPorts: rec.patchPorts,
		enableSet:  rec.enableSet,
		iniSet:     rec.iniSet,
	}
	var stdout, stderr bytes.Buffer
	if err := runReconcile(context.Background(), nil, &stdout, &stderr, deps); err != nil {
		t.Fatalf("err = %v", err)
	}
	if slices.Contains(rec.calls, "patch-game-ports") {
		t.Errorf("patch-game-ports should be skipped with asymmetric config; got %v", rec.calls)
	}
}
