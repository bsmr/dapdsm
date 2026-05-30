package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteKubeconfigContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")
	err := WriteKubeconfig(path, 9090, "vm-a.example.org", "TEFTRTY0Q0FEQVRB")
	if err != nil {
		t.Fatalf("WriteKubeconfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		"server: https://127.0.0.1:9090",
		"tls-server-name: vm-a.example.org",
		"certificate-authority-data: TEFTRTY0Q0FEQVRB",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("kubeconfig missing %q", want)
		}
	}
}

func TestWriteKubeconfigPerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := WriteKubeconfig(path, 9090, "fqdn", "ca"); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perms = %o, want 0600", info.Mode().Perm())
	}
}
