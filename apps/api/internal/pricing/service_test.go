package pricing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
)

func TestCreateOrReplaceSingleClass_DerivesTaxedFeesFromPretaxFields(t *testing.T) {
	db, countryID, userID := bootstrap(t)
	ctx := context.Background()
	repo := pricing.NewRepository(db)
	svc := pricing.NewService(repo)

	row, err := svc.CreateOrReplaceSingleClass(ctx, userID, pricing.CreateOrReplaceSingleClassRequest{
		CountryID:                      countryID,
		Continent:                      "Asia",
		CountryArea:                    "Testland",
		FirstClassFeeCNYCents:          10000,
		FirstClassFeeTax6CNYCents:      1,
		FirstClassFeeTax1CNYCents:      2,
		AdditionalClassFeeCNYCents:     20000,
		AdditionalClassFeeTax6CNYCents: 3,
		AdditionalClassFeeTax1CNYCents: 4,
		RequiredDocuments:              "Power of attorney",
		NotarizationFee:                "0",
		AcceptanceTime:                 "2 days",
		RegistrationMonths:             "6--8",
		EffectiveFrom:                  "2026-06-15",
	})
	require.NoError(t, err)

	assert.Equal(t, int64(10000), row.FirstClassFeeCNYCents)
	assert.Equal(t, int64(10600), row.FirstClassFeeTax6CNYCents)
	assert.Equal(t, int64(10100), row.FirstClassFeeTax1CNYCents)
	assert.Equal(t, int64(20000), row.AdditionalClassFeeCNYCents)
	assert.Equal(t, int64(21200), row.AdditionalClassFeeTax6CNYCents)
	assert.Equal(t, int64(20200), row.AdditionalClassFeeTax1CNYCents)

	active, err := repo.ListActiveSingleClass(ctx, pricing.SingleClassActiveFilter{
		CountryID: &countryID,
	})
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, row.ID, active[0].ID)
	assert.Equal(t, int64(10600), active[0].FirstClassFeeTax6CNYCents)
	assert.Equal(t, int64(10100), active[0].FirstClassFeeTax1CNYCents)
	assert.Equal(t, int64(21200), active[0].AdditionalClassFeeTax6CNYCents)
	assert.Equal(t, int64(20200), active[0].AdditionalClassFeeTax1CNYCents)
}
