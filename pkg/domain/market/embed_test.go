package market

import "testing"

func TestLoadCatalogEmptyPathUsesEmbedded(t *testing.T) {
	items, err := loadCatalog("")
	if err != nil {
		t.Fatalf("loadCatalog(\"\"): %v", err)
	}
	if len(items) == 0 {
		t.Fatal("embedded item-data produced no catalog items")
	}
}
