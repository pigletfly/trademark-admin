package api

import "embed"

// Migrations exposes the SQL migrations embedded into the api binary so both
// cmd/server and cmd/migrate can apply the same set without duplicating the
// //go:embed directive.
//
//go:embed all:migrations
var Migrations embed.FS
