package battlegroup

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// SetInfo summarizes one entry from .spec.serverGroup.template.spec.sets.
// Replicas is a pointer so absent ("not set") and present-zero are
// distinguishable — the Funcom CR omits the field on freshly-generated
// on-demand sets, and a JSON-Patch then needs "add" instead of "replace".
type SetInfo struct {
	Index            int    `json:"index"`
	Map              string `json:"map"`
	DedicatedScaling bool   `json:"dedicatedScaling"`
	Replicas         *int   `json:"replicas"`
	Partitions       []int  `json:"partitions,omitempty"`
}

// ListSets returns one SetInfo per element of the CR's sets array,
// preserving array order so Index aligns with JSON-Pointer paths.
func ListSets(cr []byte) ([]SetInfo, error) {
	var doc struct {
		Spec struct {
			ServerGroup struct {
				Template struct {
					Spec struct {
						Sets []json.RawMessage `json:"sets"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"serverGroup"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(cr, &doc); err != nil {
		return nil, fmt.Errorf("decode BattleGroup JSON: %w", err)
	}
	sets := doc.Spec.ServerGroup.Template.Spec.Sets
	out := make([]SetInfo, 0, len(sets))
	for i, raw := range sets {
		info, err := parseSet(i, raw)
		if err != nil {
			return nil, fmt.Errorf("set[%d]: %w", i, err)
		}
		out = append(out, info)
	}
	return out, nil
}

func parseSet(i int, raw json.RawMessage) (SetInfo, error) {
	var s struct {
		Map              string `json:"map"`
		DedicatedScaling bool   `json:"dedicatedScaling"`
		Replicas         *int   `json:"replicas"`
		Partitions       []int  `json:"partitions"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return SetInfo{}, err
	}
	return SetInfo{
		Index:            i,
		Map:              s.Map,
		DedicatedScaling: s.DedicatedScaling,
		Replicas:         s.Replicas,
		Partitions:       s.Partitions,
	}, nil
}

// BuildScalingPatches returns JSON-Patch operations that bring every set
// whose Map matches an entry in maps to (dedicatedScaling, replicas) and,
// when transitioning to always-on, also carries the correct partitions[N]
// the Funcom-Operator requires (see project_funcom-bootstrap-quirks).
//
// partitionByMap supplies that N per map, resolved by the caller from
// dune.world_partition.partition_id. When dedicatedScaling=false and the
// target set lacks a partitions field, the value from partitionByMap is
// emitted as an "add" op. When dedicatedScaling=true and a partitions
// field exists, it is removed so the set returns to the Funcom-template
// shape. Map names not present in the CR are returned in notFound; the
// caller decides whether that is fatal.
//
// Operations that would not change the existing value are omitted, so
// applying the result against a CR already in the target state is a no-op.
func BuildScalingPatches(cr []byte, maps []string, dedicatedScaling bool, replicas int, partitionByMap map[string]int) ([]Operation, []string, error) {
	sets, err := ListSets(cr)
	if err != nil {
		return nil, nil, err
	}

	indexByMap := make(map[string]int, len(sets))
	for _, s := range sets {
		indexByMap[s.Map] = s.Index
	}

	var notFound []string
	matchedIdx := make([]int, 0, len(maps))
	for _, m := range maps {
		if i, ok := indexByMap[m]; ok {
			matchedIdx = append(matchedIdx, i)
		} else {
			notFound = append(notFound, m)
		}
	}
	slices.Sort(matchedIdx)
	matchedIdx = slices.Compact(matchedIdx)

	var ops []Operation
	var missingPartition []string
	for _, i := range matchedIdx {
		s := sets[i]
		if s.DedicatedScaling != dedicatedScaling {
			ops = append(ops, Operation{
				Op:    "replace",
				Path:  fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/dedicatedScaling", i),
				Value: dedicatedScaling,
			})
		}
		if s.Replicas == nil || *s.Replicas != replicas {
			opType := "add"
			if s.Replicas != nil {
				opType = "replace"
			}
			ops = append(ops, Operation{
				Op:    opType,
				Path:  fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/replicas", i),
				Value: replicas,
			})
		}
		// partitions[] is bound to the dedicatedScaling state:
		//   on-demand (true)  → field must be absent
		//   always-on (false) → field must carry [partition_id]
		if dedicatedScaling {
			if len(s.Partitions) > 0 {
				ops = append(ops, Operation{
					Op:   "remove",
					Path: fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/partitions", i),
				})
			}
			continue
		}
		want, ok := partitionByMap[s.Map]
		if !ok {
			missingPartition = append(missingPartition, s.Map)
			continue
		}
		if slices.Equal(s.Partitions, []int{want}) {
			continue
		}
		opType := "add"
		if s.Partitions != nil {
			opType = "replace"
		}
		ops = append(ops, Operation{
			Op:    opType,
			Path:  fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/partitions", i),
			Value: []int{want},
		})
	}
	if len(missingPartition) > 0 {
		return nil, notFound, fmt.Errorf("no partition id supplied for always-on map(s): %s", strings.Join(missingPartition, ", "))
	}
	return ops, notFound, nil
}
