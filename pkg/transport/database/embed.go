package database

import (
	"embed"
	"fmt"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// q returns the embedded SQL at sql/<name>.sql; missing files panic at start.
func q(name string) string {
	b, err := sqlFS.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic(fmt.Sprintf("database: missing embedded SQL %q: %v", name, err))
	}
	return string(b)
}
