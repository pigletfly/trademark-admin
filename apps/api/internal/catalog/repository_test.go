package catalog_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("catalogtest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	mig, err := migrator.New(api.Migrations, "migrations", dsn)
	require.NoError(t, err)
	require.NoError(t, mig.Up())
	require.NoError(t, mig.Close())

	db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"))
	return db
}

func TestRepository_ListCountriesOrdered(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListCountries(context.Background(), true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 60)
	require.Equal(t, "CN", rows[0].Code) // sort_order=1
}

func TestRepository_UpdateCountry_PartialPatch(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListCountries(context.Background(), true)
	require.NoError(t, err)

	cn := rows[0]
	require.Equal(t, "CN", cn.Code)

	newDays := 45
	updated, err := repo.UpdateCountry(context.Background(), cn.ID, catalog.CountryPatch{
		DefaultAcceptanceDays: &newDays,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.DefaultAcceptanceDays)
	require.Equal(t, 45, *updated.DefaultAcceptanceDays)
	require.Equal(t, "中国", updated.NameZh) // untouched
}

func TestRepository_UpdateCountry_NotFound(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	_, err := repo.UpdateCountry(context.Background(), uuid.New(), catalog.CountryPatch{})
	require.ErrorIs(t, err, catalog.ErrNotFound)
}

func TestRepository_ListNiceCategoriesAll45(t *testing.T) {
	db := newDB(t)
	repo := catalog.NewRepository(db)

	rows, err := repo.ListNiceCategories(context.Background())
	require.NoError(t, err)
	require.Equal(t, 45, len(rows))
	require.Equal(t, 1, rows[0].Code)
	require.Equal(t, 45, rows[44].Code)
}
