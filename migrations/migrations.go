package migrations

import "embed"

// FS contains all SQL migration files embedded for runtime migration execution.
//
//go:embed *.sql
var FS embed.FS
