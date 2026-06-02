package mq

import (
	"encoding/json"
	"testing"
)

func TestBuildSkillpointsCommand(t *testing.T) {
	got := BuildSkillpointsCommand("127AC6307755DB02", 42)
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("not valid JSON: %v (%s)", err, got)
	}
	if m["ServerCommand"] != "SkillsSetUnspentSkillPoints" {
		t.Errorf("ServerCommand=%v", m["ServerCommand"])
	}
	if m["PlayerId"] != "127AC6307755DB02" {
		t.Errorf("PlayerId=%v", m["PlayerId"])
	}
	if m["SkillPoints"].(float64) != 42 {
		t.Errorf("SkillPoints=%v", m["SkillPoints"])
	}
}
