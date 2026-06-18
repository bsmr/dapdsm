package battlegroup

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Funcom's DuneSandboxServer reads its UDP listen base from
// `-ini:engine:[URL]:Port=N` and `-ini:engine:[URL]:IGWPort=M`
// passed through run.sh as CLI args. We patch those into every
// game-server set's `arguments` array so the bind port is shifted
// off Funcom's hardcoded 7777/7888 default — required when two
// BattleGroups share one public IP behind a vCD-style edge.
const (
	gamePortArgPrefix = "-ini:engine:[URL]:Port="
	igwPortArgPrefix  = "-ini:engine:[URL]:IGWPort="
)

// BuildPortPatches returns JSON-Patch operations that bring every
// game-server set in the BattleGroup CR to the requested game/IGW
// UDP base port (set via `-ini:engine:[URL]:Port=`/`IGWPort=` on
// the per-set arguments[]).
//
// Per set:
//   - missing arg → emit "add" at /arguments/-
//   - present at correct value → no op (idempotent)
//   - present at different value → emit "replace" at the existing index
//
// Sets without an arguments field at all get the args appended via "add"
// to /arguments/-; the JSON-Patch RFC permits this when the parent
// container is missing only if you create it first, but in practice
// every Funcom-generated set carries an arguments array (possibly
// empty), so we trust it.
func BuildPortPatches(cr []byte, gamePortBase, igwPortBase int) ([]Operation, error) {
	if gamePortBase <= 0 {
		return nil, fmt.Errorf("gamePortBase %d: must be > 0", gamePortBase)
	}
	if igwPortBase <= 0 {
		return nil, fmt.Errorf("igwPortBase %d: must be > 0", igwPortBase)
	}

	var doc struct {
		Spec struct {
			ServerGroup struct {
				Template struct {
					Spec struct {
						Sets []struct {
							Arguments []string `json:"arguments"`
						} `json:"sets"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"serverGroup"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(cr, &doc); err != nil {
		return nil, fmt.Errorf("decode BattleGroup JSON: %w", err)
	}

	wantGame := fmt.Sprintf("%s%d", gamePortArgPrefix, gamePortBase)
	wantIGW := fmt.Sprintf("%s%d", igwPortArgPrefix, igwPortBase)

	var ops []Operation
	for setIdx, set := range doc.Spec.ServerGroup.Template.Spec.Sets {
		ops = append(ops, portArgOpsForSet(setIdx, set.Arguments, gamePortArgPrefix, wantGame)...)
		ops = append(ops, portArgOpsForSet(setIdx, set.Arguments, igwPortArgPrefix, wantIGW)...)
	}
	return ops, nil
}

// portArgOpsForSet emits the minimal set of JSON-Patch ops to bring
// one set's arguments[] to carry exactly `wantValue` for the given
// argPrefix (e.g. "-ini:engine:[URL]:Port="). Returns nil when the
// set is already in the target state.
func portArgOpsForSet(setIdx int, args []string, argPrefix, wantValue string) []Operation {
	existingIdx := -1
	for i, a := range args {
		if strings.HasPrefix(a, argPrefix) {
			if a == wantValue {
				return nil // already correct
			}
			existingIdx = i
			break
		}
	}
	if existingIdx >= 0 {
		return []Operation{{
			Op:    "replace",
			Path:  fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/arguments/%d", setIdx, existingIdx),
			Value: wantValue,
		}}
	}
	return []Operation{{
		Op:    "add",
		Path:  fmt.Sprintf("/spec/serverGroup/template/spec/sets/%d/arguments/-", setIdx),
		Value: wantValue,
	}}
}
