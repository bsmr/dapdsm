package operatorbringup

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// webhookSecret generates a self-signed TLS keypair for an operator's admission
// webhook (CN=<op>-webhook.funcom-operators.svc) and renders it as a
// kubernetes.io/tls Secret manifest named <op>-webhook-server-cert. The private
// key never leaves this process except inside the returned manifest.
func webhookSecret(op string) ([]byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	cn := op + "-webhook." + namespace + ".svc"
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             time.Unix(0, 0),
		NotAfter:              time.Unix(0, 0).AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create cert: %w", err)
	}
	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	manifest := fmt.Sprintf("apiVersion: v1\nkind: Secret\nmetadata:\n  name: %s-webhook-server-cert\n  namespace: %s\ntype: kubernetes.io/tls\ndata:\n  tls.crt: %s\n  tls.key: %s\n",
		op, namespace, base64.StdEncoding.EncodeToString(crtPEM), base64.StdEncoding.EncodeToString(keyPEM))
	return []byte(manifest), nil
}
