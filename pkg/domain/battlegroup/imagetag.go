package battlegroup

import (
	"slices"
	"strings"
)

// PlaceholderImageTag is the tag Funcom's world template ships ({WORLD_IMAGE_TAG}
// → "0-0-shipping"); the depot images carry the real revision, so every CR
// `image` field at this tag is reconciled to <revision>-0-shipping after the
// world is created. Mirrors battlegroup.sh's update_battlegroup_to_specific_revision.
const PlaceholderImageTag = "0-0-shipping"

// BuildImageTagPatches returns one replace-op per `image` string field whose tag
// equals PlaceholderImageTag, rewriting the tag to newTag. Fields already at
// newTag (or carrying any other tag) are left untouched, so the patch is
// idempotent. The CR is walked generically (no Funcom schema knowledge): the
// world template hard-codes the placeholder tag across director / serverGateway /
// textRouter images whose paths drift between Funcom releases.
func BuildImageTagPatches(cr []byte, newTag string) ([]Operation, error) {
	root, err := decode(cr)
	if err != nil {
		return nil, err
	}
	var ops []Operation
	walk(root, nil, func(p []string, node any) {
		obj, ok := node.(map[string]any)
		if !ok {
			return
		}
		img, ok := obj["image"].(string)
		if !ok {
			return
		}
		repo, tag, found := strings.Cut(img, ":")
		if !found || tag != PlaceholderImageTag {
			return
		}
		ops = append(ops, Operation{
			Op:    "replace",
			Path:  pointer(append(slices.Clone(p), "image")),
			Value: repo + ":" + newTag,
		})
	})
	return ops, nil
}
