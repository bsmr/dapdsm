package mq

import (
	"encoding/json"
	"testing"
)

func TestBuildAddItemCommand(t *testing.T) {
	got := BuildAddItemCommand("127AC6307755DB02", "Item_Blade", 3, 1.0)
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("not valid JSON: %v (%s)", err, got)
	}
	if m["ServerCommand"] != "AddItemToInventory" {
		t.Errorf("ServerCommand=%v", m["ServerCommand"])
	}
	if m["PlayerId"] != "127AC6307755DB02" || m["ItemName"] != "Item_Blade" {
		t.Errorf("target/name wrong: %v", m)
	}
	if m["Quantity"].(float64) != 3 || m["Durability"].(float64) != 1.0 {
		t.Errorf("quantity/durability wrong: %v", m)
	}
}
