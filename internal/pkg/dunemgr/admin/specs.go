// Package admin provides a spec-driven builder and runner for in-game admin
// commands published to Funcom's RabbitMQ. Payloads are verified against the
// dune-dedicated-server-manager source (ddsm v0.3.x). Catalog validation
// (ItemName / Module / ClassName checks) is deferred to Phase 5; raw IDs are
// accepted here.
package admin

import "sort"

// fieldKind classifies how a field value is coerced.
type fieldKind int

const (
	kindString fieldKind = iota
	kindInt
	kindFloat
)

// field describes one JSON field of a ServerCommand payload.
//
//   - json:     the exact key used in the inner-JSON payload
//   - kind:     how the string value is coerced (string / int / float)
//   - required: Build returns an error if the field is absent and has no default
//   - def:      default value applied when the arg is absent; nil means omit
type field struct {
	json     string
	kind     fieldKind
	required bool
	def      any // nil → omit when absent; non-nil → apply as default
}

// spec describes one admin verb.
//
//   - verb:        dunemgr sub-verb (item, water, kick, …)
//   - command:     Funcom ServerCommand string
//   - summary:     one-line help text
//   - allowAll:    true when PlayerId="*" is permitted
//   - destructive: true when --confirm is required
//   - fields:      ordered list of payload fields (beyond ServerCommand+PlayerId)
//   - inject:      literal key→value pairs always merged into the payload
//     (e.g. AwardXP injects Category="Combat")
type spec struct {
	verb, command, summary string
	allowAll, destructive  bool
	fields                 []field
	inject                 map[string]any
}

// catalog is the full spec table, keyed by verb.
var catalog = map[string]spec{
	"item": {
		verb:     "item",
		command:  "AddItemToInventory",
		summary:  "Add an item to a player's inventory",
		allowAll: true,
		fields: []field{
			{json: "ItemName", kind: kindString, required: true},
			{json: "Quantity", kind: kindInt, def: 1},
			{json: "Durability", kind: kindFloat, def: 1.0},
		},
	},
	"water": {
		verb:     "water",
		command:  "UpdateAllWaterFillables",
		summary:  "Fill all water containers for a player",
		allowAll: true,
		fields: []field{
			{json: "WaterAmount", kind: kindInt, def: 1000000},
		},
	},
	"xp": {
		verb:     "xp",
		command:  "AwardXP",
		summary:  "Award XP to a player",
		allowAll: true,
		fields: []field{
			{json: "Experience", kind: kindInt, def: 1000},
		},
		inject: map[string]any{"Category": "Combat"},
	},
	"skill": {
		verb:     "skill",
		command:  "SkillsSetModuleLevel",
		summary:  "Set a skill module level for a player",
		allowAll: true,
		fields: []field{
			{json: "Module", kind: kindString, required: true},
			{json: "Level", kind: kindInt, def: 1},
		},
	},
	"skillpoints": {
		verb:     "skillpoints",
		command:  "SkillsSetUnspentSkillPoints",
		summary:  "Set unspent skill points for a player",
		allowAll: true,
		fields: []field{
			{json: "SkillPoints", kind: kindInt, def: 0},
		},
	},
	"vehicle": {
		verb:     "vehicle",
		command:  "SpawnVehicleAt",
		summary:  "Spawn a vehicle at coordinates for a player",
		allowAll: false,
		fields: []field{
			{json: "ClassName", kind: kindString, required: true},
			{json: "TemplateName", kind: kindString, required: true},
			{json: "X", kind: kindFloat, required: true},
			{json: "Y", kind: kindFloat, required: true},
			{json: "Z", kind: kindFloat, required: true},
			{json: "Rotation", kind: kindFloat},
			{json: "Persistent", kind: kindFloat, def: 1.0},
			{json: "Faction", kind: kindString},
		},
	},
	"teleport": {
		verb:     "teleport",
		command:  "TeleportTo",
		summary:  "Teleport a player to coordinates",
		allowAll: false,
		fields: []field{
			{json: "X", kind: kindFloat, required: true},
			{json: "Y", kind: kindFloat, required: true},
			{json: "Z", kind: kindFloat, required: true},
			{json: "Yaw", kind: kindFloat},
			{json: "CamPitch", kind: kindFloat},
			{json: "CamYaw", kind: kindFloat},
			{json: "CamRoll", kind: kindFloat},
		},
	},
	"teleport-exact": {
		verb:     "teleport-exact",
		command:  "TeleportToExact",
		summary:  "Teleport a player to exact coordinates",
		allowAll: false,
		fields: []field{
			{json: "X", kind: kindFloat, required: true},
			{json: "Y", kind: kindFloat, required: true},
			{json: "Z", kind: kindFloat, required: true},
			{json: "Yaw", kind: kindFloat},
			{json: "CamPitch", kind: kindFloat},
			{json: "CamYaw", kind: kindFloat},
			{json: "CamRoll", kind: kindFloat},
		},
	},
	"kick": {
		verb:        "kick",
		command:     "KickPlayer",
		summary:     "Kick a player from the server",
		allowAll:    true,
		destructive: true,
	},
	"clean": {
		verb:        "clean",
		command:     "CleanPlayerInventory",
		summary:     "Wipe a player's inventory",
		allowAll:    true,
		destructive: true,
	},
	"reset": {
		verb:        "reset",
		command:     "ResetProgression",
		summary:     "Reset a player's progression",
		allowAll:    true,
		destructive: true,
	},
}

// specFor looks up the spec for verb (unexported; used within the package).
func specFor(verb string) (spec, bool) {
	s, ok := catalog[verb]
	return s, ok
}

// KnownVerb reports whether verb is a registered admin verb.
// Exported for use by the command layer without exposing the full spec type.
func KnownVerb(verb string) bool {
	_, ok := catalog[verb]
	return ok
}

// Verbs returns the sorted list of registered verbs.
func Verbs() []string {
	out := make([]string, 0, len(catalog))
	for v := range catalog {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
