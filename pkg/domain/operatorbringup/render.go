// Package operatorbringup brings the four Funcom operators up on a multi-node
// cluster: it renders their Deployments, generates webhook TLS secrets,
// optionally installs cert-manager, labels the workers, applies the operator
// CRDs, and waits for Available. It ports scripts/dune-bootstrap-kubernetes.sh
// steps 5-10 for multi-node RKE2. See the S3 spec.
// It ends at operators Available; FLS + BattleGroup are ds-bashar.
package operatorbringup

import (
	"bytes"
	"fmt"
	"text/template"
)

// renderOperators renders the embedded operator manifest (4 ServiceAccounts +
// 4 Deployments + declarative RBAC) with the operator version substituted in.
// Images keep their original registry.funcom.com refs and resolve locally via
// IfNotPresent (loaded by `ds-arrakis images load`) — no registry rewrite.
func renderOperators(version string) ([]byte, error) {
	t, err := template.New("operators").Parse(operatorsTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse operators template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, struct{ Version string }{version}); err != nil {
		return nil, fmt.Errorf("render operators manifest: %w", err)
	}
	return buf.Bytes(), nil
}
