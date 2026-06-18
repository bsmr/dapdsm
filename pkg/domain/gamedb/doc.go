// Package gamedb binds the Funcom game DB's stored functions and the
// project's own SQL into typed Go calls over the kubectl-exec psql wire.
//
// SQL lives in sql/<name>.sql (go:embed) and is loaded via q("name"); Go
// code holds only the result types, psql :var binding, and stdout parsing.
// To add a binding: drop sql/<name>.sql, add a result struct, and write a
// method that runs q("name") through the Runner and scans the | output.
//
// Exceptions kept inline: SQL composed from shared fragments (pawnSubquery,
// itemPawnScope) and SQL built with fmt.Sprintf over dynamic identifiers
// (schema column listing, slow-query report).
package gamedb
