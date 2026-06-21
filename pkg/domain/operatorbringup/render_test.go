package operatorbringup

import (
	"strings"
	"testing"
)

func TestRenderOperatorsKeepsOriginalRefs(t *testing.T) {
	m, err := renderOperators("1.5.0")
	if err != nil {
		t.Fatalf("renderOperators: %v", err)
	}
	s := string(m)
	for _, op := range []string{"battlegroup", "database", "server", "utilities"} {
		want := "image: registry.funcom.com/funcom/self-hosting/igw-k8s-" + op + "-operator:1.5.0"
		if !strings.Contains(s, want) {
			t.Errorf("missing original ref %q", want)
		}
	}
	if strings.Contains(s, "{{") {
		t.Errorf("unrendered template directive remains:\n%s", s)
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
