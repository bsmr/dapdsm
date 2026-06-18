package mq

import "encoding/json"

// BuildAwardXPCommand returns the inner ServerCommand JSON for awarding
// specialization-track XP to an online player, matching dune-admin's AwardXP wire
// format (Category = track name). Wrap + publish via PublishInner.
func BuildAwardXPCommand(playerFLS, category string, experience int64) string {
	cmd := struct {
		ServerCommand string `json:"ServerCommand"`
		PlayerId      string `json:"PlayerId"`
		Category      string `json:"Category"`
		Experience    int64  `json:"Experience"`
	}{
		ServerCommand: "AwardXP",
		PlayerId:      playerFLS,
		Category:      category,
		Experience:    experience,
	}
	b, _ := json.Marshal(cmd)
	return string(b)
}
