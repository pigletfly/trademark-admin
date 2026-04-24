// Command seed upserts the embedded catalog JSON into the configured database.
// Idempotent; safe to run repeatedly. Used for manual re-seeds after an
// operator updates the JSON files in-repo.
package main

import (
	"context"
	"os"
	"time"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

func main() {
	log := logger.New("info", "development")

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("open db", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"); err != nil {
		log.Error("seed", "error", err)
		os.Exit(1)
	}
	log.Info("seed complete")
}
