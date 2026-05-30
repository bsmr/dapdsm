package kube

import (
	"fmt"
	"os"
)

// WriteKubeconfig writes a throwaway kubeconfig YAML to path,
// pointing at https://127.0.0.1:<localPort>, with tls-server-name
// set to fqdn (matching the SAN on the K3s cert) and the CA in
// base64. File mode is 0600.
//
// The resulting file is intended to be passed via KUBECONFIG to
// per-call kubectl invocations.
func WriteKubeconfig(path string, localPort int, fqdn, caB64 string) error {
	body := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:%d
    tls-server-name: %s
    certificate-authority-data: %s
  name: dunemgr
contexts:
- context:
    cluster: dunemgr
    user: dunemgr
  name: dunemgr
current-context: dunemgr
users:
- name: dunemgr
  user: {}
`, localPort, fqdn, caB64)
	return os.WriteFile(path, []byte(body), 0o600)
}
