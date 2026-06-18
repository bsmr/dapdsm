package mq

import "encoding/json"

// BuildSkillpointsCommand returns the inner ServerCommand JSON for setting a
// player's unspent skill points to an absolute value, matching dune-admin's
// SkillsSetUnspentSkillPoints wire format. The presence-aware give-skillpoints
// online path computes the absolute target (current unspent + delta) and passes
// it here. Wrap + publish via PublishInner (heartbeats/notifications).
func BuildSkillpointsCommand(playerFLS string, unspent int64) string {
	cmd := struct {
		ServerCommand string `json:"ServerCommand"`
		PlayerId      string `json:"PlayerId"`
		SkillPoints   int64  `json:"SkillPoints"`
	}{
		ServerCommand: "SkillsSetUnspentSkillPoints",
		PlayerId:      playerFLS,
		SkillPoints:   unspent,
	}
	b, _ := json.Marshal(cmd)
	return string(b)
}
