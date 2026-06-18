package dbquery

import "encoding/json"

// Progression holds the FLevelComponent fields surfaced by player inspect.
type Progression struct {
	TotalSkillPoints         int64
	UnspentSkillPoints       int64
	TotalXPEarned            int64
	KeystoneBonusSkillPoints int64
	RespecCostCounter        int64
	ActivePerks              int
	ActiveAbilities          int
}

// Vitals holds the FHealthComponent fields.
type Vitals struct {
	CurrentHealth float64
	DownCurrent   float64
	DownMax       float64
}

// SpiceState holds the FSpiceAddictionComponent string fields.
type SpiceState struct {
	SystemStatus string
	SpiceVision  string
}

// componentElem1 extracts element [1] (the data object) of a `[0, {...}]`
// component array from the top-level components JSON.
func componentElem1(components []byte, key string) (json.RawMessage, bool) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(components, &top); err != nil {
		return nil, false
	}
	raw, ok := top[key]
	if !ok {
		return nil, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) < 2 {
		return nil, false
	}
	return arr[1], true
}

func parseProgression(components []byte) (Progression, bool) {
	elem, ok := componentElem1(components, "FLevelComponent")
	if !ok {
		return Progression{}, false
	}
	var v struct {
		TotalSkillPoints         int64             `json:"TotalSkillPoints"`
		UnspentSkillPoints       int64             `json:"UnspentSkillPoints"`
		TotalXPEarned            int64             `json:"TotalXPEarned"`
		KeystoneBonusSkillPoints int64             `json:"KeystoneBonusSkillPoints"`
		RespecCostCounter        int64             `json:"RespecCostCounter"`
		ActivePerkTags           []json.RawMessage `json:"ActivePerkTags"`
		ActiveAbilityTags        []json.RawMessage `json:"ActiveAbilityTags"`
	}
	if err := json.Unmarshal(elem, &v); err != nil {
		return Progression{}, false
	}
	return Progression{
		TotalSkillPoints:         v.TotalSkillPoints,
		UnspentSkillPoints:       v.UnspentSkillPoints,
		TotalXPEarned:            v.TotalXPEarned,
		KeystoneBonusSkillPoints: v.KeystoneBonusSkillPoints,
		RespecCostCounter:        v.RespecCostCounter,
		ActivePerks:              len(v.ActivePerkTags),
		ActiveAbilities:          len(v.ActiveAbilityTags),
	}, true
}

func parseVitals(components []byte) (Vitals, bool) {
	elem, ok := componentElem1(components, "FHealthComponent")
	if !ok {
		return Vitals{}, false
	}
	var v struct {
		Cur     float64 `json:"m_CurrentHealth"`
		DownCur float64 `json:"m_CurrentDownButNotOutStateHealth"`
		DownMax float64 `json:"m_MaxDownButNotOutStateHealth"`
	}
	if err := json.Unmarshal(elem, &v); err != nil {
		return Vitals{}, false
	}
	return Vitals{CurrentHealth: v.Cur, DownCurrent: v.DownCur, DownMax: v.DownMax}, true
}

func parseSpice(components []byte) (SpiceState, bool) {
	elem, ok := componentElem1(components, "FSpiceAddictionComponent")
	if !ok {
		return SpiceState{}, false
	}
	var v struct {
		SystemStatus string `json:"SystemStatus"`
		SpiceVision  string `json:"SpiceVisionEnabledStatus"`
	}
	if err := json.Unmarshal(elem, &v); err != nil {
		return SpiceState{}, false
	}
	return SpiceState{SystemStatus: v.SystemStatus, SpiceVision: v.SpiceVision}, true
}
