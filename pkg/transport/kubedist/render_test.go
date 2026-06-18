package kubedist

import (
	"strings"
	"testing"
)

func TestRenderDropins_OmitsInternalRoutingWhenSame(t *testing.T) {
	got, err := renderDropins("k3s", Config{ExternalIP: "1.2.3.4", InternalIP: "1.2.3.4", TLSSANs: []string{"a.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["30-internal-routing.yaml"]; ok {
		t.Fatal("30-internal-routing must be omitted when internal == external")
	}
	if !strings.Contains(string(got["20-external-ip.yaml"]), "node-external-ip: 1.2.3.4") {
		t.Fatalf("external-ip drop-in wrong: %s", got["20-external-ip.yaml"])
	}
	if !strings.Contains(string(got["10-host.yaml"]), "- a.example") {
		t.Fatalf("tls-san drop-in wrong: %s", got["10-host.yaml"])
	}
}

func TestRenderDropins_IncludesInternalRoutingWhenDNAT(t *testing.T) {
	got, err := renderDropins("k3s", Config{ExternalIP: "9.9.9.9", InternalIP: "10.0.0.5"})
	if err != nil {
		t.Fatal(err)
	}
	body := string(got["30-internal-routing.yaml"])
	for _, want := range []string{"node-ip: 10.0.0.5", "advertise-address: 10.0.0.5", "bind-address: 10.0.0.5"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestBaseConfig_BothDistros(t *testing.T) {
	for _, d := range []string{"k3s", "rke2"} {
		b, err := baseConfig(d)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s baseConfig: %v len=%d", d, err, len(b))
		}
	}
}
