package pricing_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// bootstrap spins up Postgres, runs all migrations, and seeds a country
// + admin user so pricing_entries FKs are satisfied.
func bootstrap(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("pricing_test"),
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

	// Look up admin role. IMPORTANT: Scan into a string first — GORM+pgx
	// returns UUID columns as text strings, and [16]byte cannot be
	// assigned from a string directly.
	var roleIDStr string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "admin").Scan(&roleIDStr).Error)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	// Insert a country so pricing_entries.country_id FK is satisfied.
	countryID := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO countries (id, code, name_zh, name_en, sort_order, enabled)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		countryID, "TT", "测试国", "Testland", 0, true,
	).Error)

	// Insert admin user.
	userID := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, "Pricing Test", "pricing-test-"+userID.String()+"@test.local", "hash", roleID,
	).Error)

	return db, countryID, userID
}

func TestReplaceActive_InsertsNewAndDeprecatesOld(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	e1, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 10000,
		EffectiveFrom:  day1,
		CreatedBy:      userID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e1.ID)
	assert.Nil(t, e1.EffectiveTo)

	// Replace — must deprecate e1 and insert e2 atomically.
	e2, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 12000,
		EffectiveFrom:  day2,
		CreatedBy:      userID,
	})
	require.NoError(t, err)
	assert.Nil(t, e2.EffectiveTo)

	hist, err := repo.ListHistory(ctx, pricing.HistoryFilter{
		CountryID:   countryID,
		ServiceTier: "basic",
		FeeItem:     "application",
	})
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, e2.ID, hist[0].ID)
	assert.Nil(t, hist[0].EffectiveTo)
	assert.Equal(t, e1.ID, hist[1].ID)
	require.NotNil(t, hist[1].EffectiveTo)
	// After Postgres round-trip, DATE values come back at 00:00 UTC, so
	// compare in UTC to be safe.
	assert.True(t, hist[1].EffectiveTo.UTC().Equal(day2.UTC()),
		"old entry's effective_to should equal new's effective_from; got %s want %s",
		hist[1].EffectiveTo.UTC(), day2.UTC())

	active, err := repo.ListActive(ctx, pricing.ActiveFilter{CountryID: &countryID})
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, e2.ID, active[0].ID)
}

func TestDeprecate_AlreadyDeprecated(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	e1, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "renewal",
		AmountCNYCents: 5000,
		EffectiveFrom:  day1,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	_, err = repo.Deprecate(ctx, e1.ID, day2)
	require.NoError(t, err)

	// Second deprecate must error.
	_, err = repo.Deprecate(ctx, e1.ID, day2.Add(24*time.Hour))
	assert.ErrorIs(t, err, pricing.ErrNoActive)
}

func TestDeprecate_EffectiveToMustBeAfterFrom(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	e, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "standard",
		FeeItem:        "x",
		AmountCNYCents: 1000,
		EffectiveFrom:  from,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	// effective_to = effective_from is invalid
	_, err = repo.Deprecate(ctx, e.ID, from)
	require.Error(t, err)

	// past date also invalid
	_, err = repo.Deprecate(ctx, e.ID, from.Add(-24*time.Hour))
	require.Error(t, err)
}

func TestListActive_FiltersByTier(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, tier := range []string{"basic", "standard", "premium"} {
		_, err := repo.ReplaceActive(ctx, pricing.NewEntry{
			CountryID:      countryID,
			ServiceTier:    tier,
			FeeItem:        "application",
			AmountCNYCents: 1000,
			EffectiveFrom:  from,
			CreatedBy:      userID,
		})
		require.NoError(t, err)
	}

	tier := "standard"
	got, err := repo.ListActive(ctx, pricing.ActiveFilter{
		CountryID:   &countryID,
		ServiceTier: &tier,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "standard", got[0].ServiceTier)
}

func TestReplaceActiveSingleClass_InsertsNewAndDeprecatesOld(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	first, err := repo.ReplaceActiveSingleClass(ctx, pricing.NewSingleClassEntry{
		CountryID:                      countryID,
		Continent:                      "Asia",
		CountryArea:                    "Testland",
		FirstClassFeeCNYCents:          360000,
		FirstClassFeeTax6CNYCents:      381600,
		FirstClassFeeTax1CNYCents:      363600,
		AdditionalClassFeeCNYCents:     270000,
		AdditionalClassFeeTax6CNYCents: 286200,
		AdditionalClassFeeTax1CNYCents: 272700,
		RequiredDocuments:              "Power of attorney",
		NotarizationFee:                "0",
		AcceptanceTime:                 "2 days",
		RegistrationMonths:             "6--8",
		EffectiveFrom:                  day1,
		CreatedBy:                      userID,
	})
	require.NoError(t, err)

	second, err := repo.ReplaceActiveSingleClass(ctx, pricing.NewSingleClassEntry{
		CountryID:                      countryID,
		Continent:                      "Asia",
		CountryArea:                    "Testland",
		FirstClassFeeCNYCents:          380000,
		FirstClassFeeTax6CNYCents:      402800,
		FirstClassFeeTax1CNYCents:      383800,
		AdditionalClassFeeCNYCents:     290000,
		AdditionalClassFeeTax6CNYCents: 307400,
		AdditionalClassFeeTax1CNYCents: 292900,
		RequiredDocuments:              "Power of attorney",
		NotarizationFee:                "0",
		AcceptanceTime:                 "2 days",
		RegistrationMonths:             "6--8",
		EffectiveFrom:                  day2,
		CreatedBy:                      userID,
	})
	require.NoError(t, err)

	active, err := repo.ListActiveSingleClass(ctx, pricing.SingleClassActiveFilter{CountryID: &countryID})
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, second.ID, active[0].ID)
	assert.Equal(t, int64(380000), active[0].FirstClassFeeCNYCents)

	gotOld, err := repo.GetSingleClassByID(ctx, first.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOld.EffectiveTo)
	assert.True(t, gotOld.EffectiveTo.UTC().Equal(day2.UTC()))
}

func TestReplaceActiveMadrid_VersionsBaseAndCountryRowsSeparately(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	base, err := repo.ReplaceActiveMadrid(ctx, pricing.NewMadridEntry{
		CountryArea:         "Basic registration fee - black and white mark",
		OfficialFeeCHFCents: 65300,
		AgencyFeeCNYCents:   400000,
		IsBaseFee:           true,
		EffectiveFrom:       day1,
		CreatedBy:           userID,
	})
	require.NoError(t, err)
	countryRow, err := repo.ReplaceActiveMadrid(ctx, pricing.NewMadridEntry{
		CountryID:           &countryID,
		SequenceNo:          pricingTestIntPtr(1),
		CountryArea:         "Testland",
		OfficialFeeCHFCents: 26100,
		AgencyFeeCNYCents:   40000,
		EffectiveFrom:       day1,
		CreatedBy:           userID,
	})
	require.NoError(t, err)
	updatedBase, err := repo.ReplaceActiveMadrid(ctx, pricing.NewMadridEntry{
		CountryArea:         "Basic registration fee - black and white mark",
		OfficialFeeCHFCents: 90300,
		AgencyFeeCNYCents:   500000,
		IsBaseFee:           true,
		EffectiveFrom:       day2,
		CreatedBy:           userID,
	})
	require.NoError(t, err)

	active, err := repo.ListActiveMadrid(ctx, pricing.MadridActiveFilter{
		CountryID:   &countryID,
		IncludeBase: true,
	})
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, updatedBase.ID, active[0].ID)
	assert.True(t, active[0].IsBaseFee)
	assert.Equal(t, countryRow.ID, active[1].ID)

	gotOldBase, err := repo.GetMadridByID(ctx, base.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOldBase.EffectiveTo)
	assert.True(t, gotOldBase.EffectiveTo.UTC().Equal(day2.UTC()))
}

func pricingTestIntPtr(v int) *int {
	return &v
}

// TestRepo_GetByID_ReturnsDeprecatedEntry locks in the M4 assumption
// that GetByID does NOT filter by effective_to — historical lookup
// needs to reach rows even after they've been deprecated by a newer
// version.
func TestRepo_GetByID_ReturnsDeprecatedEntry(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)

	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	old, err := repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 10000,
		EffectiveFrom:  day1,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	// Replace deprecates `old` by setting effective_to = day2.
	_, err = repo.ReplaceActive(ctx, pricing.NewEntry{
		CountryID:      countryID,
		ServiceTier:    "basic",
		FeeItem:        "application",
		AmountCNYCents: 12000,
		EffectiveFrom:  day2,
		CreatedBy:      userID,
	})
	require.NoError(t, err)

	// GetByID on the now-deprecated old entry must still return it.
	got, err := repo.GetByID(ctx, old.ID)
	require.NoError(t, err)
	assert.Equal(t, old.ID, got.ID)
	require.NotNil(t, got.EffectiveTo)
	assert.Equal(t, int64(10000), got.AmountCNYCents)
}

// TestRepo_GetByID_NotFound returns ErrNotFound for a random UUID.
func TestRepo_GetByID_NotFound(t *testing.T) {
	db, _, _ := bootstrap(t)
	repo := pricing.NewRepository(db)
	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, pricing.ErrNotFound)
}
