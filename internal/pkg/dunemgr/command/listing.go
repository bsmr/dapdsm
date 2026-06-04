package command

// ListingHeaderLines reports how many leading lines of a verb's stdout are a
// sticky table header (which the TUI pins while the data scrolls). Only the
// tabular listings carry one; everything else returns 0.
func ListingHeaderLines(argv []string) int {
	if len(argv) < 3 {
		return 0
	}
	switch argv[0] {
	case "player":
		if argv[2] == "search" {
			return 1
		}
	case "avatar":
		if argv[2] == "list" || argv[2] == "exports" {
			return 1
		}
	}
	return 0
}
