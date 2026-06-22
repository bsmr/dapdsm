package battlegroup

import (
	"context"
	"encoding/json"
	"fmt"

	"go.muehmer.eu/dapdsm/pkg/transport/kube"
)

// bgResource is the kubectl resource name for the Funcom BattleGroup CR.
const bgResource = "battlegroup"

// stopPatch builds the merge-patch that toggles the BattleGroup's stop field.
//
// LIVE-VERIFY (A1): the field is assumed to be spec.stop (boolean). If the
// operator's CR uses a different name/shape, change ONLY this function.
func stopPatch(stop bool) string {
	b, _ := json.Marshal(map[string]any{"spec": map[string]any{"stop": stop}})
	return string(b)
}

// Stop sets the BattleGroup's stop field true (the operator drains the
// server group). K8s-native equivalent of the vendor wrapper's `stop`.
func Stop(ctx context.Context, r kube.Runner, ns, bg string) error {
	return r.Patch(ctx, bgResource, bg, ns, "merge", stopPatch(true))
}

// Start clears the stop field; the operator brings servers back per scaling.
func Start(ctx context.Context, r kube.Runner, ns, bg string) error {
	return r.Patch(ctx, bgResource, bg, ns, "merge", stopPatch(false))
}

// Restart stops then starts. Gating between the two (waiting for the stop to
// take effect) is the orchestrator's job, not the primitive's.
func Restart(ctx context.Context, r kube.Runner, ns, bg string) error {
	if err := Stop(ctx, r, ns, bg); err != nil {
		return err
	}
	return Start(ctx, r, ns, bg)
}

// Update reconciles every placeholder image tag on the CR to newTag (the
// already-staged depot revision) and applies the result as a JSON patch.
// No-op when nothing matches.
//
// LIVE-VERIFY (A2): this reuses BuildImageTagPatches, which rewrites only
// fields still at PlaceholderImageTag. A real-revision→real-revision retag
// needs a generalized builder; whether that is required (vs. the operator
// re-pulling on its own) is confirmed on dune-01 before generalizing here.
func Update(ctx context.Context, r kube.Runner, ns, bg, newTag string) error {
	cr, err := r.Get(ctx, bgResource, bg, "-n", ns, "-o", "json")
	if err != nil {
		return err
	}
	ops, err := BuildImageTagPatches(cr, newTag)
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		return nil
	}
	b, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("marshal image-tag patch: %w", err)
	}
	return r.Patch(ctx, bgResource, bg, ns, "json", string(b))
}
