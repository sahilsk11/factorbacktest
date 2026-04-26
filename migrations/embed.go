// Package migrations exposes the SQL migration files as an embedded FS so a
// production binary (cmd/migrate) can apply them without the migrations/
// directory needing to ship alongside it. The embed declaration has to live
// here because go:embed patterns are resolved relative to the source file and
// can't traverse upward with "..".
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
