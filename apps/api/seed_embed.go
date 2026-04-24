package api

import "embed"

// SeedFS exposes all seed JSON files as an embedded read-only file system.
// Consumers (pkg/seeder, cmd/seed) read from this to keep the binary
// hermetic and avoid path discovery at runtime.
//
//go:embed all:seed
var SeedFS embed.FS
