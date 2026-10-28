// Package migrations holds embedded SQL migrations for goose.
package migrations

import "embed"

// FS contains the SQL migration files.
//
//go:embed *.sql
var FS embed.FS
