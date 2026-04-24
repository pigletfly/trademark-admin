//go:build integration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
)

func TestOpenAndPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// postgres.BasicWaitStrategies() waits for "database system is ready to
	// accept connections" twice, which is the reliable signal that postgres
	// is fully ready. Required explicitly in testcontainers-go v0.42+.
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("tm"),
		postgres.WithUsername("tm"),
		postgres.WithPassword("tm"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres.Run: %v", err)
	}
	t.Cleanup(func() {
		_ = ctr.Terminate(ctx)
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.Ping(ctx, db); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
