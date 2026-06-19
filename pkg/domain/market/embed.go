package market

import _ "embed"

// embeddedItemData is the vendored item-data.json (see PROVENANCE.md). It is the
// default catalog source when BotConfig.ItemDataPath is empty.
//
//go:embed item-data.json
var embeddedItemData []byte
