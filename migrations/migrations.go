package migrations

import "embed"

// FS contains all SQL migration files embedded for runtime migration execution.
// Organized by database type: sqlite/ and postgres/ subdirectories
//
//go:embed sqlite/*.sql postgres/*.sql
var FS embed.FS
