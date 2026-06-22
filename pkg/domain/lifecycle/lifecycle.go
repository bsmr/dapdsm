// Package lifecycle runs BG lifecycle verbs (start/stop/restart/update)
// against a host by shelling out to the Funcom vendor wrapper over SSH.
//
// Deprecated for multi-node: this is the single-node (on-VM) path. The
// K8s-native lifecycle (CR patches that work through a jumphost) lives in
// pkg/domain/battlegroup (Stop/Start/Restart/Update) and
// pkg/domain/bgorchestrator (Upgrade). New multi-node code should use those.
package lifecycle

import "fmt"

// DefaultBattlegroupBin is the location of Funcom's vendor wrapper
// on the host. The wrapper handles multi-step orchestration that a
// plain BG-CR patch cannot model (Steam re-pull for update,
// image-tag rewrite, partition restart ordering).
const DefaultBattlegroupBin = "/home/dune/.dune/bin/battlegroup"

// Action is one of the four supported lifecycle verbs.
type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionUpdate  Action = "update"
)

// Valid reports whether a is one of the four recognized actions.
func (a Action) Valid() bool {
	switch a {
	case ActionStart, ActionStop, ActionRestart, ActionUpdate:
		return true
	}
	return false
}

// ValidateAction parses s into a recognized Action or returns an
// error suitable for surfacing to operators / CLI argv.
func ValidateAction(s string) (Action, error) {
	a := Action(s)
	if !a.Valid() {
		return "", fmt.Errorf("lifecycle action %q: must be one of start|stop|restart|update", s)
	}
	return a, nil
}
