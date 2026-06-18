package mq

import (
	"encoding/json"
	"testing"
)

func TestBuildAwardXPCommand(t *testing.T) {
	got := BuildAwardXPCommand("127AC6307755DB02", "Combat", 1500)
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("not valid JSON: %v (%s)", err, got)
	}
	if m["ServerCommand"] != "AwardXP" || m["PlayerId"] != "127AC6307755DB02" {
		t.Errorf("command/target wrong: %v", m)
	}
	if m["Category"] != "Combat" || m["Experience"].(float64) != 1500 {
		t.Errorf("category/experience wrong: %v", m)
	}
}
