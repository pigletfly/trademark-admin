package seeder_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

// newTestDB spins up a pg container, runs embedded migrations and returns a GORM handle.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("seedertest"),
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
	return db
}

// minimalFS contains only the two seed files we feed to seeder.Run.
func minimalFS() fstest.MapFS {
	return fstest.MapFS{
		"seed/countries.json": &fstest.MapFile{Data: []byte(`[
			{"code":"CN","name_zh":"中国","name_en":"China","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":12,"requires_notarization":false,"sort_order":1},
			{"code":"US","name_zh":"美国","name_en":"United States","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":10}
		]`)},
		"seed/nice_categories.json": &fstest.MapFile{Data: []byte(`[
			{"code":1,"name_zh":"化学品","name_en":"Chemicals"},
			{"code":35,"name_zh":"广告商业","name_en":"Advertising"}
		]`)},
	}
}

func TestRun_InsertsThenUpdatesIdempotently(t *testing.T) {
	db := newTestDB(t)
	fs := minimalFS()

	// First run: insert.
	require.NoError(t, seeder.Run(context.Background(), db, fs, "seed/countries.json", "seed/nice_categories.json"))

	var countCountries, countCategories int64
	require.NoError(t, db.Table("countries").Count(&countCountries).Error)
	require.NoError(t, db.Table("nice_categories").Count(&countCategories).Error)
	require.EqualValues(t, 2, countCountries)
	require.EqualValues(t, 2, countCategories)

	// Second run with same data: no duplicates.
	require.NoError(t, seeder.Run(context.Background(), db, fs, "seed/countries.json", "seed/nice_categories.json"))
	require.NoError(t, db.Table("countries").Count(&countCountries).Error)
	require.EqualValues(t, 2, countCountries)

	// Mutate seed data and re-run: existing row updates.
	modified := fstest.MapFS{
		"seed/countries.json": &fstest.MapFile{Data: []byte(`[
			{"code":"CN","name_zh":"中国-Updated","name_en":"China","is_madrid_member":true,"default_acceptance_days":45,"default_registration_months":15,"requires_notarization":false,"sort_order":1},
			{"code":"US","name_zh":"美国","name_en":"United States","is_madrid_member":true,"default_acceptance_days":30,"default_registration_months":10,"requires_notarization":false,"sort_order":10}
		]`)},
		"seed/nice_categories.json": &fstest.MapFile{Data: []byte(`[
			{"code":1,"name_zh":"化学品","name_en":"Chemicals"},
			{"code":35,"name_zh":"广告商业","name_en":"Advertising"}
		]`)},
	}
	require.NoError(t, seeder.Run(context.Background(), db, modified, "seed/countries.json", "seed/nice_categories.json"))

	var cn struct {
		NameZh                string
		DefaultAcceptanceDays int
	}
	require.NoError(t, db.Table("countries").Select("name_zh, default_acceptance_days").Where("code = ?", "CN").Scan(&cn).Error)
	require.Equal(t, "中国-Updated", cn.NameZh)
	require.EqualValues(t, 45, cn.DefaultAcceptanceDays)
}

func TestRun_WithRealEmbeddedFS(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, seeder.Run(context.Background(), db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json"))

	var countryCount, catCount int64
	require.NoError(t, db.Table("countries").Count(&countryCount).Error)
	require.NoError(t, db.Table("nice_categories").Count(&catCount).Error)
	require.GreaterOrEqual(t, countryCount, int64(60))
	require.EqualValues(t, 45, catCount)
}
