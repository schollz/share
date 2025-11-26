package migrations

import "embed"

// FS contains all SQL migration files embedded for runtime migration execution.
// Organized by database type: postgres/ subdirectory only
//
//go:embed postgres/*.sql
var FS embed.FS
