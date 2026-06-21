package worldsetup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// jwt builds a token "<hdr>.<payload>.<sig>" with the given payload JSON.
func jwt(payloadJSON string) []byte {
	enc := base64.RawURLEncoding.EncodeToString
	return []byte(enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(payloadJSON)) + ".sig")
}

func TestParseFLSHostID_Lowercases(t *testing.T) {
	id, err := ParseFLSHostID(jwt(`{"HostId":"AbCdEf12"}`))
	if err != nil {
		t.Fatalf("ParseFLSHostID: %v", err)
	}
	if id != "abcdef12" {
		t.Errorf("HostId = %q, want %q", id, "abcdef12")
	}
}

func TestParseFLSHostID_BadToken(t *testing.T) {
	for _, tok := range [][]byte{[]byte("notajwt"), []byte("a.b"), jwt(`{"NoHost":1}`)} {
		if _, err := ParseFLSHostID(tok); err == nil {
			t.Errorf("ParseFLSHostID(%q) = nil error, want error", tok)
		}
	}
}

func TestUniqueName_Format(t *testing.T) {
	// 6 zero bytes → 6× index-0 letter 'a'.
	name, err := uniqueName("abc123", bytes.NewReader(make([]byte, 6)))
	if err != nil {
		t.Fatalf("uniqueName: %v", err)
	}
	if name != "sh-abc123-aaaaaa" {
		t.Errorf("uniqueName = %q, want %q", name, "sh-abc123-aaaaaa")
	}
}

func TestRMQSecret_Base64Of64Bytes(t *testing.T) {
	s, err := rmqSecret(bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("rmqSecret: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(raw) != 64 {
		t.Errorf("rmqSecret not base64 of 64 bytes: len=%d err=%v", len(raw), err)
	}
}

func TestPassword_AlnumLen(t *testing.T) {
	p, err := password(bytes.NewReader(make([]byte, 24)), 24)
	if err != nil {
		t.Fatalf("password: %v", err)
	}
	if len(p) != 24 || strings.Trim(p, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		t.Errorf("password = %q, want 24 alnum chars", p)
	}
}

type fakeSeam struct {
	files     map[string]string
	kubectl   [][]string
	stdin     [][]byte
	stdinArgs [][]string
}

func (f *fakeSeam) ReadDepotFile(_ context.Context, p string) ([]byte, error) {
	v, ok := f.files[p]
	if !ok {
		return nil, fmt.Errorf("no such depot file: %s", p)
	}
	return []byte(v), nil
}
func (f *fakeSeam) Kubectl(_ context.Context, args ...string) (string, error) {
	f.kubectl = append(f.kubectl, args)
	return "", nil
}
func (f *fakeSeam) KubectlStdin(_ context.Context, in []byte, args ...string) (string, error) {
	f.stdin = append(f.stdin, in)
	f.stdinArgs = append(f.stdinArgs, args)
	return "", nil
}

func TestCreateWorld_NamespaceAndCreates(t *testing.T) {
	reader = bytes.NewReader(make([]byte, 256)) // deterministic; restore via t.Cleanup
	t.Cleanup(func() { reader = rand.Reader })
	dir := "/home/dune/depot/prod"
	tmplDir := dir + "/scripts/setup/templates"
	s := &fakeSeam{files: map[string]string{
		tmplDir + "/world-template.yaml": "world: {WORLD_NAME} tag: {WORLD_IMAGE_TAG}",
		tmplDir + "/fls-secret.yaml":     "fls: {FLS_SECRET}",
		tmplDir + "/rmq-secret.yaml":     "rmq: {RMQ_SECRET}",
	}}
	res, err := CreateWorld(context.Background(), s,
		Config{WorldName: "MyBG", WorldRegion: "Europe", DepotDir: dir},
		jwt(`{"HostId":"ABC"}`))
	if err != nil {
		t.Fatalf("CreateWorld: %v", err)
	}
	if res.Namespace != "funcom-seabass-"+res.UniqueName {
		t.Errorf("Namespace = %q, want funcom-seabass-%s", res.Namespace, res.UniqueName)
	}
	if !strings.HasPrefix(res.UniqueName, "sh-abc-") {
		t.Errorf("UniqueName = %q, want sh-abc-…", res.UniqueName)
	}
	// 1 create-namespace via Kubectl, 3 create -f - via KubectlStdin.
	if len(s.kubectl) != 1 || s.kubectl[0][0] != "create" || s.kubectl[0][1] != "namespace" {
		t.Errorf("want one `create namespace`, got %v", s.kubectl)
	}
	if len(s.stdinArgs) != 3 {
		t.Fatalf("want 3 `create -f -` calls, got %d", len(s.stdinArgs))
	}
	// CR carries the placeholder tag and the WorldName as a YAML-quoted scalar
	// (the title slot is bare in the template), and targets the ns.
	if !bytes.Contains(s.stdin[2], []byte(`world: "MyBG"`)) ||
		!bytes.Contains(s.stdin[2], []byte("tag: 0-0-shipping")) {
		t.Errorf("CR stdin not rendered: %s", s.stdin[2])
	}
}
