package catalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func setup(t *testing.T) (*gin.Engine, *catalog.Service, *gorm.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("catalogh"),
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

	repo := catalog.NewRepository(db)
	svc := catalog.NewService(repo)
	h := catalog.NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	// For tests we skip auth/CSRF and mount both groups on the root router.
	v1 := router.Group("/api/v1")
	catalog.RegisterReadRoutes(v1, h)
	catalog.RegisterAdminRoutes(v1, h)
	return router, svc, db
}

func TestGET_Countries_ReturnsSeeded(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/countries", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct{ Items []catalog.CountryDTO }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.GreaterOrEqual(t, len(body.Items), 60)
	require.Equal(t, "CN", body.Items[0].Code)
}

func TestGET_NiceCategories_45Rows(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/nice-categories", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct{ Items []catalog.NiceCategoryDTO }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 45, len(body.Items))
}

func TestPATCH_Country_PartialUpdate(t *testing.T) {
	router, svc, _ := setup(t)
	countries, err := svc.ListCountries(context.Background(), false)
	require.NoError(t, err)
	cn := countries[0]

	newDays := 99
	body, _ := json.Marshal(catalog.UpdateCountryRequest{DefaultAcceptanceDays: &newDays})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/"+cn.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got catalog.CountryDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.NotNil(t, got.DefaultAcceptanceDays)
	require.Equal(t, 99, *got.DefaultAcceptanceDays)
	require.Equal(t, cn.NameZh, got.NameZh)
}

func TestPATCH_Country_NotFound(t *testing.T) {
	router, _, _ := setup(t)
	body, _ := json.Marshal(catalog.UpdateCountryRequest{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPATCH_Country_BadUUID(t *testing.T) {
	router, _, _ := setup(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/countries/not-a-uuid", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
