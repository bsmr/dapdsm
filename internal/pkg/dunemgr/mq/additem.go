package mq

import "encoding/json"

// BuildAddItemCommand returns the inner ServerCommand JSON for granting an item to
// an online player, matching the dune-admin / ddsm wire format. Wrap it with
// EncodeEnvelope + publish via PublishInner (heartbeats/notifications). The MQ path
// has no quality field; durability defaults to 1.0 at the call site.
func BuildAddItemCommand(playerFLS, itemName string, quantity int64, durability float64) string {
	cmd := struct {
		ServerCommand string  `json:"ServerCommand"`
		PlayerId      string  `json:"PlayerId"`
		ItemName      string  `json:"ItemName"`
		Quantity      int64   `json:"Quantity"`
		Durability    float64 `json:"Durability"`
	}{
		ServerCommand: "AddItemToInventory",
		PlayerId:      playerFLS,
		ItemName:      itemName,
		Quantity:      quantity,
		Durability:    durability,
	}
	b, _ := json.Marshal(cmd)
	return string(b)
}
