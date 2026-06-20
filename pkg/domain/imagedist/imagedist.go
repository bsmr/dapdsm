// Package imagedist distributes the Funcom depot images (staged on the jumphost
// by the depot slice) into the cluster via a persistent in-cluster registry:
// Deploy stands the registry up, Push copies the tars into it from the jumphost.
// Nodes then pull through the normal containerd path (the provisioner seeds
// registries.yaml). See the S2 image-distribution spec.
package imagedist

import (
	"context"
	"fmt"
	"strings"

	"go.muehmer.eu/dapdsm/pkg/domain/depot"
	"go.muehmer.eu/dapdsm/pkg/transport/skopeo"
)

// ImageRef is a pushed image reference (registry prefix omitted — Push records
// the original repo:tag; the registry is in Result.Registry). Digest-pinning is
// deferred (spec open item 3); S3 rewrites operator image fields by repo:tag.
type ImageRef struct {
	Repo string
	Tag  string
}

// Result is the outcome of a Push: the target registry and the images pushed.
type Result struct {
	Registry string
	Images   []ImageRef
}

// Push ensures skopeo is installed, then for every *.tar under the depot's three
// image dirs reads its RepoTags and copies each image into registry. It records
// the pushed refs in encounter order. registry is a host:port endpoint.
func Push(ctx context.Context, r skopeo.Runner, registry string, d depot.Result) (Result, error) {
	if err := skopeo.EnsureInstalled(ctx, r); err != nil {
		return Result{}, fmt.Errorf("imagedist: %w", err)
	}
	res := Result{Registry: registry}
	for _, dir := range []string{d.OperatorsDir, d.PrerequisitesDir, d.BattlegroupDir} {
		tars, err := listTars(ctx, r, dir)
		if err != nil {
			return Result{}, err
		}
		for _, tar := range tars {
			tags, err := skopeo.RepoTags(ctx, r, tar)
			if err != nil {
				return Result{}, err
			}
			for _, repoTag := range tags {
				src := fmt.Sprintf("docker-archive:%s:%s", tar, repoTag)
				dst := fmt.Sprintf("docker://%s/%s", registry, repoTag)
				if err := skopeo.Copy(ctx, r, src, dst); err != nil {
					return Result{}, err
				}
				res.Images = append(res.Images, splitRef(repoTag))
			}
		}
	}
	return res, nil
}

// listTars lists *.tar files in dir on the host via the runner. A dir with no
// tars yields no entries (not an error — a depot env may lack a group).
func listTars(ctx context.Context, r skopeo.Runner, dir string) ([]string, error) {
	out, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("ls -1 '%s'/*.tar 2>/dev/null || true", dir))
	if err != nil {
		return nil, fmt.Errorf("list tars in %s: %w", dir, err)
	}
	var tars []string
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			tars = append(tars, s)
		}
	}
	return tars, nil
}

// splitRef splits a "repo:tag" reference at the last colon.
func splitRef(repoTag string) ImageRef {
	if i := strings.LastIndex(repoTag, ":"); i >= 0 {
		return ImageRef{Repo: repoTag[:i], Tag: repoTag[i+1:]}
	}
	return ImageRef{Repo: repoTag}
}
