package command

import (
	"sort"

	"go.muehmer.eu/dapdsm/internal/pkg/dunemgr/admin"
	admincatalog "go.muehmer.eu/dapdsm/pkg/domain/catalog"
	"go.muehmer.eu/dapdsm/pkg/domain/gameini"
)

// argKind classifies a positional argument for completion + help.
type argKind int

const (
	argFixed   argKind = iota // complete against a fixed options set (sub-verbs)
	argHost                   // complete against the known host names
	argFree                   // freeform (bg name, sql, flags) — no completion
	argCatalog                // complete against a vendored catalog (items/skills/vehicles)
	argPlayer                 // completed from the TUI's live per-host player-name cache
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
	// SubArgs optionally overrides the slots that follow the argFixed sub
	// slot, keyed by the sub-verb token. When set and the typed sub-verb is a
	// key, positions after the sub slot resolve from SubArgs[sub] instead of
	// Args. Used by verbs whose later slots depend on the sub-verb (currently
	// admin and avatar). Verbs with positionally-fixed slots leave it nil.
	SubArgs map[string][]argSlot
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
	"item": {
		Verb: "item", Summary: "Edit/delete a single inventory item by id (offline player)",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"set", "delete"}, name: "sub"},
			{kind: argFree, name: "item-id"},
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
	"give": {
		Verb: "give", Summary: "Grant currency/item/skillpoints/xp/charxp to a player (presence-aware)",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"currency", "item", "skillpoints", "xp", "charxp"}, name: "sub"},
		},
		SubArgs: map[string][]argSlot{
			"currency":    {{kind: argPlayer, name: "player"}, {kind: argFree, name: "currency-id"}, {kind: argFree, name: "delta"}},
			"item":        {{kind: argPlayer, name: "player"}, {kind: argCatalog, catalogKey: "items", name: "template"}, {kind: argFree, name: "count"}},
			"skillpoints": {{kind: argPlayer, name: "player"}, {kind: argFree, name: "amount"}},
			"xp":          {{kind: argPlayer, name: "player"}, {kind: argFree, name: "amount"}},
			"charxp":      {{kind: argPlayer, name: "player"}, {kind: argFree, name: "amount"}},
		},
	},
	"player": {
		Verb: "player", Summary: "Look up / inspect a player by name or FLS-ID",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"search", "pos", "inspect"}, name: "sub"},
			{kind: argPlayer, name: "query"},
		},
	},
	"avatar": {
		Verb: "avatar", Summary: "Export / list / import / transfer a single player's avatar",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: []string{"export", "list", "exports", "import", "transfer"}, name: "sub"},
		},
		SubArgs: map[string][]argSlot{
			"export":   {{kind: argPlayer, name: "player"}},
			"list":     {},
			"exports":  {},
			"import":   {{kind: argPlayer, name: "player"}, {kind: argFree, name: "key"}},
			"transfer": {{kind: argHost, name: "dst-host"}, {kind: argPlayer, name: "player"}},
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
		Verb: "whisper", Summary: "Send a private in-game chat message to one player (--from spoof or --as GM|Server)",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argPlayer, name: "player"},
			{kind: argFree, name: "message"},
		},
	},
}

// init registers the "admin" spec after the admin package verb list is
// available. Every admin verb takes a player slot; item/skill/vehicle add a
// catalog name slot. Other verbs expose no catalog (so the suggestion line is
// not flooded with irrelevant items).
func init() {
	sub := make(map[string][]argSlot, len(admin.Verbs()))
	for _, v := range admin.Verbs() {
		slots := []argSlot{{kind: argPlayer, name: "player-id"}}
		switch v {
		case "item":
			slots = append(slots, argSlot{kind: argCatalog, catalogKey: "items", name: "name"})
		case "skill":
			slots = append(slots, argSlot{kind: argCatalog, catalogKey: "skills", name: "name"})
		case "vehicle":
			slots = append(slots, argSlot{kind: argCatalog, catalogKey: "vehicles", name: "name"})
		}
		sub[v] = slots
	}
	specs["admin"] = Spec{
		Verb:    "admin",
		Summary: "Publish an in-game admin command to a player via MQ",
		Args: []argSlot{
			{kind: argHost, name: "host"},
			{kind: argFixed, options: admin.Verbs(), name: "verb"},
		},
		SubArgs: sub,
	}
}

// FirstArgIsHost reports whether the verb's first positional is a host slot.
func (s Spec) FirstArgIsHost() bool {
	return len(s.Args) > 0 && s.Args[0].kind == argHost
}

// fixedSlotIndex returns the index of the argFixed slot in Args, or -1.
func (s Spec) fixedSlotIndex() int {
	for i, a := range s.Args {
		if a.kind == argFixed {
			return i
		}
	}
	return -1
}

// slotAt returns the effective argSlot at pos (0-based into the logical arg
// list, i.e. command-line token index minus one), given the already-typed
// tokens (tokens[0] is the verb). When the verb declares SubArgs and the
// sub-verb token names a key, positions after the argFixed sub slot resolve
// from SubArgs; otherwise from Args. ok is false for an out-of-range pos.
func (s Spec) slotAt(pos int, tokens []string) (argSlot, bool) {
	if pos < 0 {
		return argSlot{}, false
	}
	if s.SubArgs != nil {
		if sub := s.fixedSlotIndex(); sub >= 0 {
			if len(tokens) > sub+1 {
				if subSlots, ok := s.SubArgs[tokens[sub+1]]; ok {
					if pos <= sub {
						if pos < len(s.Args) {
							return s.Args[pos], true
						}
						return argSlot{}, false
					}
					rel := pos - (sub + 1)
					if rel < len(subSlots) {
						return subSlots[rel], true
					}
					return argSlot{}, false
				}
			}
		}
	}
	if pos < len(s.Args) {
		return s.Args[pos], true
	}
	return argSlot{}, false
}

// IsCatalogPos reports whether the slot at pos is catalog-backed, given the
// typed tokens (needed for SubArgs verbs). The TUI suppresses suggestions on
// an empty token for these slots.
func (s Spec) IsCatalogPos(pos int, tokens ...string) bool {
	slot, ok := s.slotAt(pos, tokens)
	return ok && slot.kind == argCatalog
}

// IsPlayerPos reports whether the slot at pos is a player-name slot, given the
// typed tokens. The TUI serves these from its live per-host name cache.
func (s Spec) IsPlayerPos(pos int, tokens ...string) bool {
	slot, ok := s.slotAt(pos, tokens)
	return ok && slot.kind == argPlayer
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
		case argPlayer:
			out += " <" + a.name + ">"
		}
	}
	return out
}

// Candidates returns the completion pool for the slot at pos given the known
// hosts and the typed tokens. nil for freeform/player/out-of-range slots.
func (s Spec) Candidates(pos int, hosts []string, tokens ...string) []string {
	slot, ok := s.slotAt(pos, tokens)
	if !ok {
		return nil
	}
	switch slot.kind {
	case argFixed:
		return append([]string(nil), slot.options...)
	case argHost:
		return append([]string(nil), hosts...)
	case argCatalog:
		return catalogCandidates(slot.catalogKey)
	default: // argFree, argPlayer
		return nil
	}
}

// catalogCandidates resolves a catalog key to its vendored candidate list.
func catalogCandidates(key string) []string {
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
