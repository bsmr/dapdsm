package operatorbringup

import (
	"strings"
	"testing"
)

func TestRenderOperators_SubstitutesAndRewrites(t *testing.T) {
	out, err := renderOperators("v1.5.0", "10.0.0.9:5000")
	if err != nil {
		t.Fatalf("renderOperators: %v", err)
	}
	s := string(out)
	// version + registry substituted into every operator image
	for _, op := range []string{"battlegroup", "database", "server", "utilities"} {
		want := "image: 10.0.0.9:5000/funcom/self-hosting/igw-k8s-" + op + "-operator:v1.5.0"
		if !strings.Contains(s, want) {
			t.Errorf("missing rewritten image %q", want)
		}
	}
	// no residual upstream registry or version placeholder
	if strings.Contains(s, "registry.funcom.com") {
		t.Error("residual registry.funcom.com — ref-rewrite incomplete")
	}
	if strings.Contains(s, "__OPERATOR_VERSION__") {
		t.Error("residual __OPERATOR_VERSION__ placeholder")
	}
	// all 4 deployments + SAs + RBAC present
	for _, op := range []string{"battlegroupoperator", "databaseoperator", "serveroperator", "utilitiesoperator"} {
		for _, want := range []string{
			"name: " + op + "-controller-manager",
			"name: " + op + "-manager-rolebinding",
			"name: " + op + "-leader-election-role",
		} {
			if !strings.Contains(s, want) {
				t.Errorf("op %s: missing %q", op, want)
			}
		}
	}
	// webhook cert volumes preserved verbatim
	if !strings.Contains(s, "secretName: battlegroupoperator-webhook-server-cert") {
		t.Error("webhook cert secret reference lost")
	}
}
