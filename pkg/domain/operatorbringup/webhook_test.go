package operatorbringup

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func TestWebhookSecret_RendersValidTLSSecret(t *testing.T) {
	out, err := webhookSecret("battlegroupoperator")
	if err != nil {
		t.Fatalf("webhookSecret: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"kind: Secret",
		"type: kubernetes.io/tls",
		"name: battlegroupoperator-webhook-server-cert",
		"namespace: funcom-operators",
		"tls.crt:",
		"tls.key:",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("secret manifest missing %q", want)
		}
	}
	// the embedded cert must be a real, parseable x509 cert with the right CN
	crtB64 := fieldValue(t, s, "tls.crt:")
	der, err := base64.StdEncoding.DecodeString(crtB64)
	if err != nil {
		t.Fatalf("tls.crt not valid base64: %v", err)
	}
	block, _ := pem.Decode(der)
	if block == nil {
		t.Fatal("tls.crt is not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if cert.Subject.CommonName != "battlegroupoperator-webhook.funcom-operators.svc" {
		t.Errorf("CN = %q", cert.Subject.CommonName)
	}
}

// fieldValue extracts the base64 value after a `  key:` line in the manifest.
func fieldValue(t *testing.T, manifest, key string) string {
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key))
		}
	}
	t.Fatalf("key %q not found in manifest", key)
	return ""
}
