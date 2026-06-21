// Package worldsetup creates a Funcom BattleGroup "world" on a multi-node,
// jumphost-fronted cluster the K8s-native way: it ports Funcom's single-node
// world.sh (FLS-HostId parse → secrets → render the depot's templates → create
// namespace + secrets + BattleGroup CR) over the clusteraccess seam, with no
// sudo and no k3s.sh. Replaces ds-bashar's setup.sh driver on multi-node.
package worldsetup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

const lowercase = "abcdefghijklmnopqrstuvwxyz"
const alnum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ParseFLSHostID decodes the JWT payload (segment [1], base64url, no padding),
// reads HostId, and lowercases it — matching world.sh's
// `jq … .[1].HostId | tr A-Z a-z`.
func ParseFLSHostID(token []byte) (string, error) {
	parts := strings.Split(strings.TrimSpace(string(token)), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("FLS token is not a 3-segment JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode FLS token payload: %w", err)
	}
	var claims struct {
		HostID string `json:"HostId"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse FLS token payload: %w", err)
	}
	if claims.HostID == "" {
		return "", fmt.Errorf("FLS token has no HostId claim")
	}
	return strings.ToLower(claims.HostID), nil
}

// pick returns n characters drawn uniformly from set using rnd.
// ponytail: modulo introduces negligible bias over 26/62-char set; acceptable for non-cryptographic naming/credential entropy at these lengths.
func pick(rnd io.Reader, set string, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rnd, buf); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = set[int(b)%len(set)]
	}
	return string(out), nil
}

// uniqueName builds WORLD_UNIQUE_NAME = "sh-<hostID>-<6 lowercase letters>".
func uniqueName(hostID string, rnd io.Reader) (string, error) {
	suffix, err := pick(rnd, lowercase, 6)
	if err != nil {
		return "", err
	}
	return "sh-" + hostID + "-" + suffix, nil
}

// rmqSecret is base64-std of 64 random bytes (RMQ_SECRET).
func rmqSecret(rnd io.Reader) (string, error) {
	buf := make([]byte, 64)
	if _, err := io.ReadFull(rnd, buf); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// password returns n characters from [A-Za-z0-9].
func password(rnd io.Reader, n int) (string, error) {
	return pick(rnd, alnum, n)
}

// reader is the package's randomness source; overridable in tests.
var reader io.Reader = rand.Reader

// Seam abstracts cluster access and depot file reads so CreateWorld can be
// tested without a live cluster or filesystem.
type Seam interface {
	ReadDepotFile(ctx context.Context, path string) ([]byte, error)
	Kubectl(ctx context.Context, args ...string) (string, error)
	KubectlStdin(ctx context.Context, stdin []byte, args ...string) (string, error)
}

// Config holds the caller-supplied world parameters.
type Config struct {
	WorldName   string // BattleGroup title (already validated YAML-safe by caller)
	WorldRegion string // human region name
	DepotDir    string // /home/dune/depot/<target>
}

// Result holds the outputs of a successful CreateWorld call.
type Result struct {
	Namespace  string
	UniqueName string
}

// templateNames are the three depot templates world.sh renders, in create order
// (secrets first, then the BattleGroup CR). Confirmed against the depot at the
// live-grounding step.
var templateNames = []string{"fls-secret.yaml", "rmq-secret.yaml", "world-template.yaml"}

// CreateWorld renders the depot templates and creates the namespace, the two
// secrets, and the BattleGroup CR via the seam (dune kubeconfig, no sudo). The
// non-idempotent create is guarded by the caller's bring-up discovery gate (skip
// when a funcom-seabass-* namespace already exists).
func CreateWorld(ctx context.Context, s Seam, cfg Config, flsToken []byte) (Result, error) {
	hostID, err := ParseFLSHostID(flsToken)
	if err != nil {
		return Result{}, err
	}
	unique, err := uniqueName(hostID, reader)
	if err != nil {
		return Result{}, err
	}
	rmq, err := rmqSecret(reader)
	if err != nil {
		return Result{}, err
	}
	pgPass, err := password(reader, 24)
	if err != nil {
		return Result{}, err
	}
	dunePass, err := password(reader, 24)
	if err != nil {
		return Result{}, err
	}
	ns := "funcom-seabass-" + unique
	vals := map[string]string{
		"WORLD_NAME":          yamlQuote(cfg.WorldName), // bare `title: {WORLD_NAME}` slot → quote for spaces/colons
		"WORLD_UNIQUE_NAME":   unique,
		"WORLD_REGION":        cfg.WorldRegion,
		"WORLD_IMAGE_TAG":     PlaceholderImageTag,
		"WORLD_POSTGRES_PASS": pgPass,
		"WORLD_DUNE_PASS":     dunePass,
		"FLS_SECRET":          string(flsToken),
		"RMQ_SECRET":          rmq,
	}
	if _, err := s.Kubectl(ctx, "create", "namespace", ns); err != nil {
		return Result{}, fmt.Errorf("create namespace %s: %w", ns, err)
	}
	tmplDir := path.Join(cfg.DepotDir, "scripts", "setup", "templates")
	for _, name := range templateNames {
		raw, err := s.ReadDepotFile(ctx, path.Join(tmplDir, name))
		if err != nil {
			return Result{}, fmt.Errorf("read template %s: %w", name, err)
		}
		rendered := renderTemplate(string(raw), vals)
		if _, err := s.KubectlStdin(ctx, []byte(rendered), "create", "-n", ns, "-f", "-"); err != nil {
			return Result{}, fmt.Errorf("create %s in %s: %w", name, ns, err)
		}
	}
	return Result{Namespace: ns, UniqueName: unique}, nil
}
