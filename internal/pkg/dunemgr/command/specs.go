package command

import "sort"

// argKind classifies a positional argument for completion + help.
type argKind int

const (
	argFixed argKind = iota // complete against a fixed options set (sub-verbs)
	argHost                 // complete against the known host names
	argFree                 // freeform (bg name, sql, flags) — no completion
)

// argSlot describes one positional argument of a verb.
type argSlot struct {
	kind    argKind
	options []string // for argFixed
	name    string   // display name: "host", "bg", "sql", "flags"
}

// Spec is the completion + help metadata for one dispatcher verb. It is the
// single source of truth shared by the CLI dispatcher and the TUI command bar,
// independent of any frontend.
type Spec struct {
	Verb    string
	Summary string
	Args    []argSlot
}

// specs maps verb -> Spec. Keep in sync with the dispatch table.
var specs = map[string]Spec{
	"host": {
		Verb: "host", Summary: "Manage the host pool",
		Args: []argSlot{{kind: argFixed, options: []string{"add", "list", "rm", "probe"}, name: "sub"}},
	},
	"lifecycle": {
		Verb: "lifecycle", Summary: "Drive a BattleGroup lifecycle verb",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"start", "stop", "restart", "update"}, name: "action"},
		},
	},
	"broadcast": {
		Verb: "broadcast", Summary: "Publish an in-game notice or shutdown announcement",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"notice", "shutdown", "shutdown-cancel"}, name: "kind"},
			{kind: argFree, name: "flags"},
		},
	},
	"db": {
		Verb: "db", Summary: "Run a read-only DB query",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"exec", "columns", "slow"}, name: "sub"},
			{kind: argFree, name: "args"},
		},
	},
	"backup": {
		Verb: "backup", Summary: "Create / list / restore BattleGroup DB backups",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFree, name: "bg"},
			{kind: argFixed, options: []string{"create", "list", "restore"}, name: "sub"},
			{kind: argFree, name: "args"},
		},
	},
	"shutdown": {
		Verb: "shutdown", Summary: "Schedule / cancel / inspect a shutdown countdown",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"schedule", "cancel", "status"}, name: "sub"},
			{kind: argFree, name: "flags"},
		},
	},
}

// Specs returns all verb specs, sorted by verb.
func Specs() []Spec {
	out := make([]Spec, 0, len(specs))
	for _, s := range specs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Verb < out[j].Verb })
	return out
}

// SpecFor returns the spec for a verb.
func SpecFor(verb string) (Spec, bool) {
	s, ok := specs[verb]
	return s, ok
}

// Usage renders a one-line usage string for a verb from its spec, e.g.
// "lifecycle <host> [start|stop|restart|update]".
func (s Spec) Usage() string {
	out := s.Verb
	for _, a := range s.Args {
		switch a.kind {
		case argFixed:
			out += " ["
			for i, o := range a.options {
				if i > 0 {
					out += "|"
				}
				out += o
			}
			out += "]"
		case argHost:
			out += " <host>"
		case argFree:
			out += " <" + a.name + ">"
		}
	}
	return out
}

// Candidates returns the completion pool for the positional argument at index
// pos (0-based into Args; i.e. command-line token position minus one), given the
// known host names. It returns nil for a freeform slot or an out-of-range pos.
// This keeps the unexported argKind/options inside command — the TUI never needs
// to know the slot kinds, only the resulting candidate strings.
func (s Spec) Candidates(pos int, hosts []string) []string {
	if pos < 0 || pos >= len(s.Args) {
		return nil
	}
	switch a := s.Args[pos]; a.kind {
	case argFixed:
		return append([]string(nil), a.options...)
	case argHost:
		return append([]string(nil), hosts...)
	default: // argFree
		return nil
	}
}
