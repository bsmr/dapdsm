package dbquery

import (
	"embed"
	"fmt"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// q returns the embedded SQL statement at sql/<name>.sql. A missing file
// panics at process start (the embed list is fixed at build time), so a
// typo fails tests, never production.
func q(name string) string {
	b, err := sqlFS.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic(fmt.Sprintf("dbquery: missing embedded SQL %q: %v", name, err))
	}
	return string(b)
}
