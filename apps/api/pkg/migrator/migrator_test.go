//go:build integration

package migrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func TestMigratorUpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

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
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}

	m, err := migrator.New(api.Migrations, "migrations", dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	if err := m.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version == 0 {
		t.Fatalf("expected non-zero version after Up, got %d (dirty=%v)", version, dirty)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
}
