package command

import (
	"sort"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin"
	admincatalog "go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin/catalog"
	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/gameini"
)

// argKind classifies a positional argument for completion + help.
type argKind int

const (
	argFixed   argKind = iota // complete against a fixed options set (sub-verbs)
	argHost                   // complete against the known host names
	argFree                   // freeform (bg name, sql, flags) — no completion
	argCatalog                // complete against a vendored catalog (items/skills/vehicles)
)

// argSlot describes one positional argument of a verb.
type argSlot struct {
	kind       argKind
	options    []string // for argFixed
	name       string   // display name: "host", "bg", "sql", "flags"
	catalogKey string   // for argCatalog: "items", "skills", or "vehicles"
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
// The "admin" entry is populated by init() so that admin.Verbs() drives the
// options list without duplicating the verb list here.
var specs = map[string]Spec{
	"host": {
		Verb: "host", Summary: "Manage the host pool",
		Args: []argSlot{{kind: argFixed, options: []string{"add", "list", "rm", "probe"}, name: "sub"}},
	},
	"ini": {
		Verb: "ini", Summary: "Read/set curated gameplay settings (vendor UserEngine.ini)",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"list", "get", "set"}, name: "sub"},
			{kind: argCatalog, catalogKey: "ini", name: "key"},
		},
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
	"player": {
		Verb: "player", Summary: "Look up / inspect a player by name or FLS-ID",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"search", "pos", "inspect"}, name: "sub"},
			{kind: argFree, name: "query"},
		},
	},
	"avatar": {
		Verb: "avatar", Summary: "Export / list / import / transfer a single player's avatar",
		Args: []argSlot{
			{kind: argFixed, options: []string{"export", "list", "import", "transfer"}, name: "sub"},
			{kind: argHost, name: "host"},
			// Slot 2 is the dst-host for `transfer` and the fls-id for the
			// other sub-verbs. Marked argHost so `transfer` gets host
			// completion on both hosts; on export/list/import the host hints
			// on the fls slot are harmless and ignorable (cf. admin's catalog
			// default on its name slot).
			{kind: argHost, name: "host-or-fls"},
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
	"stats": {
		Verb: "stats", Summary: "Show node telemetry (CPU / memory / disk / network)",
		Args: []argSlot{{kind: argHost, name: "host"}},
	},
	"whisper": {
		Verb: "whisper", Summary: "Send a private in-game chat message to one player",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFree, name: "fls-id"},
			{kind: argFree, name: "message"},
		},
	},
}

// init registers the "admin" spec after the admin package verb list is available.
func init() {
	specs["admin"] = Spec{
		Verb:    "admin",
		Summary: "Publish an in-game admin command to a player via MQ",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: admin.Verbs(), name: "verb"},
			{kind: argFree, name: "player-id"},
			// The name arg (ItemName/Module/ClassName) is catalog-backed.
			// Completion peeks at the typed verb token (tokens[1] if present)
			// to select the right catalog. Verbs without a catalog name arg
			// (kick, clean, reset, water, xp, skillpoints, teleport*) default
			// to items — the operator sees a suggestion list but can ignore it.
			{kind: argCatalog, catalogKey: "items", name: "name"},
		},
	}
}

// IsCatalogPos reports whether the argument slot at pos (0-based) is a
// catalog-backed slot. The TUI uses this to suppress suggestions on empty tokens
// (the catalog may have thousands of entries).
func (s Spec) IsCatalogPos(pos int) bool {
	if pos < 0 || pos >= len(s.Args) {
		return false
	}
	return s.Args[pos].kind == argCatalog
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
		case argCatalog:
			out += " <" + a.name + ">"
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
//
// tokens is an optional variadic parameter holding the already-typed prefix
// tokens (i.e. everything before the current in-progress token). It is used by
// argCatalog slots to select the right catalog based on the typed verb token
// (tokens[1] for the admin verb, which sits at arg index 0 = tokens[0] being
// the outer verb, tokens[1] being the admin sub-verb).
func (s Spec) Candidates(pos int, hosts []string, tokens ...string) []string {
	if pos < 0 || pos >= len(s.Args) {
		return nil
	}
	switch a := s.Args[pos]; a.kind {
	case argFixed:
		return append([]string(nil), a.options...)
	case argHost:
		return append([]string(nil), hosts...)
	case argCatalog:
		return catalogCandidates(a.catalogKey, tokens)
	default: // argFree
		return nil
	}
}

// catalogCandidates resolves the catalog key, optionally overriding it based
// on the admin sub-verb found in tokens[2] (the third typed token). The tokens
// slice passed here is the full prefix-token list for the line:
//
//	tokens[0] = outer verb ("admin")
//	tokens[1] = host
//	tokens[2] = admin sub-verb ("item", "skill", "vehicle", …)
//	tokens[3] = player-id
//
// Falls back to the slot's declared catalogKey (items) for verbs without a
// catalog name arg.
func catalogCandidates(defaultKey string, tokens []string) []string {
	key := defaultKey
	// adminSubVerb is at index 2 in the full token list.
	if len(tokens) > 2 {
		switch tokens[2] {
		case "skill":
			key = "skills"
		case "vehicle":
			key = "vehicles"
		case "item":
			key = "items"
		}
	}
	switch key {
	case "skills":
		return admincatalog.SkillIDs()
	case "vehicles":
		return admincatalog.VehicleIDs()
	case "ini":
		return gameini.Keys()
	default: // "items" or unknown
		return admincatalog.ItemIDs()
	}
}
