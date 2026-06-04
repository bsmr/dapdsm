// Package catalog embeds and exposes the vendored item, skill-module, and
// vehicle catalogs sourced from adainrivers/dune-dedicated-server-manager
// (© 2026 gaming.tools, MIT — see PROVENANCE.md).
//
// All three catalogs are parsed once at program start via package-level vars.
// The exposed accessors are goroutine-safe (read-only after init).
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed items.json
var rawItems []byte

//go:embed skill-modules.json
var rawSkills []byte

//go:embed vehicles.json
var rawVehicles []byte

//go:embed stack-max.json
var rawStackMax []byte

// Item is one entry from items.json.
type Item struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Source   string `json:"source"`
}

// Skill is one entry from skill-modules.json.
type Skill struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	MaxLevel int    `json:"maxLevel"`
}

// Vehicle is one entry from vehicles.json.
type Vehicle struct {
	ID         string   `json:"id"`
	ActorClass string   `json:"actor_class"`
	Templates  []string `json:"templates"`
}

// package-level parsed catalogs, initialised once.
var (
	allItems    []Item
	allSkills   []Skill
	allVehicles []Vehicle

	itemIndex     map[string]struct{}
	nameIndex     map[string]string
	skillIndex    map[string]Skill
	vehicleIndex  map[string]Vehicle
	stackMaxIndex map[string]int
)

func init() {
	mustParseInto(rawItems, &allItems, "items.json")
	mustParseInto(rawSkills, &allSkills, "skill-modules.json")
	mustParseInto(rawVehicles, &allVehicles, "vehicles.json")

	itemIndex = make(map[string]struct{}, len(allItems))
	nameIndex = make(map[string]string, len(allItems))
	for _, it := range allItems {
		itemIndex[it.ID] = struct{}{}
		nameIndex[it.ID] = it.Name
	}

	skillIndex = make(map[string]Skill, len(allSkills))
	for _, sk := range allSkills {
		skillIndex[sk.ID] = sk
	}

	vehicleIndex = make(map[string]Vehicle, len(allVehicles))
	for _, v := range allVehicles {
		vehicleIndex[v.ID] = v
	}

	if err := json.Unmarshal(rawStackMax, &stackMaxIndex); err != nil {
		panic(fmt.Sprintf("catalog: parse stack-max.json: %v", err))
	}
}

func mustParseInto(data []byte, dst any, name string) {
	if err := json.Unmarshal(data, dst); err != nil {
		panic(fmt.Sprintf("catalog: failed to parse %s: %v", name, err))
	}
}

// Items returns all item entries.
func Items() []Item { return allItems }

// Skills returns all skill-module entries.
func Skills() []Skill { return allSkills }

// Vehicles returns all vehicle entries.
func Vehicles() []Vehicle { return allVehicles }

// ItemIDs returns all item IDs in catalog order.
func ItemIDs() []string {
	ids := make([]string, len(allItems))
	for i, it := range allItems {
		ids[i] = it.ID
	}
	return ids
}

// SkillIDs returns all skill-module IDs in catalog order.
func SkillIDs() []string {
	ids := make([]string, len(allSkills))
	for i, sk := range allSkills {
		ids[i] = sk.ID
	}
	return ids
}

// VehicleIDs returns all vehicle IDs in catalog order.
func VehicleIDs() []string {
	ids := make([]string, len(allVehicles))
	for i, v := range allVehicles {
		ids[i] = v.ID
	}
	return ids
}

// SkillMaxLevel returns the maximum level for the given skill module ID.
// ok is false when the ID is not in the catalog.
func SkillMaxLevel(id string) (int, bool) {
	sk, ok := skillIndex[id]
	if !ok {
		return 0, false
	}
	return sk.MaxLevel, true
}

// VehicleTemplates returns the valid template names for the given vehicle ID.
// ok is false when the ID is not in the catalog.
func VehicleTemplates(id string) ([]string, bool) {
	v, ok := vehicleIndex[id]
	if !ok {
		return nil, false
	}
	return v.Templates, true
}

// DisplayName returns the human display name for an item template id, or the id
// unchanged when the catalog has no entry (graceful fallback).
func DisplayName(id string) string {
	if n, ok := nameIndex[id]; ok && n != "" {
		return n
	}
	return id
}

// StackMax returns the maximum stack size for an item template, or 0 if unknown.
func StackMax(id string) int { return stackMaxIndex[id] }
