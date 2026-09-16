// Package migrations embeds the .sql schema files so the binary carries its own schema.
//
// Embedding rather than shipping a directory next to the executable matters for packaging:
// the app is a single self-contained binary, and a schema that could go missing beside it
// would turn a version skew into a startup failure.
package migrations

import "embed"

// FS holds every migration, named NNNN_name.sql, at its root. Read it with
// migrate.Load / migrate.Up.
//
//go:embed *.sql
var FS embed.FS
