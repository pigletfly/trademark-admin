package customer_test

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
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// setupHandler brings up pg + migrations, seeds a user with the requested
// role, mounts customer routes with a fake auth middleware that injects
// the test user into the Gin context (bypassing JWT/CSRF).
func setupHandler(t *testing.T, role string) (*gin.Engine, *gorm.DB, uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("custh"),
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

	var roleIDStr string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", role).Scan(&roleIDStr).Error)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)

	user := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		user, "Test Owner", "u-"+user.String()+"@test.local", "hash", roleID,
	).Error)

	repo := customer.NewRepository(db)
	svc := customer.NewService(repo)
	h := customer.NewHandler(svc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	// Fake auth middleware — real CSRF/JWT tested elsewhere.
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: user, Role: role})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	return router, db, user
}

func do(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// lookupRoleID — scans UUID via string to avoid pgx text/uuid conversion issue.
func lookupRoleID(t *testing.T, db *gorm.DB, code string) uuid.UUID {
	t.Helper()
	var s string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", code).Scan(&s).Error)
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

func TestCreate_Then_Get(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)

	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"})
	require.Equal(t, http.StatusCreated, w.Code)
	var created customer.CustomerDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.Equal(t, "Acme", created.Name)

	w = do(t, r, http.MethodGet, "/api/v1/customers/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestCreate_DuplicateName_409(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)
	require.Equal(t, http.StatusCreated, do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"}).Code)
	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Acme"})
	require.Equal(t, http.StatusConflict, w.Code)
}

func TestPatch_OwnerGuard_ForbidsCrossOwner(t *testing.T) {
	// Owner 1 creates a row.
	r1, db, _ := setupHandler(t, customer.RoleSalesperson)
	w := do(t, r1, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "Alpha"})
	require.Equal(t, http.StatusCreated, w.Code)
	var created customer.CustomerDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// Owner 2 on the same DB.
	roleID := lookupRoleID(t, db, customer.RoleSalesperson)
	otherUser := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		otherUser, "Other", "other-"+otherUser.String()+"@test.local", "h", roleID,
	).Error)
	repo := customer.NewRepository(db)
	h := customer.NewHandler(customer.NewService(repo))
	r2 := gin.New()
	v1 := r2.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: otherUser, Role: customer.RoleSalesperson})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	newName := "Beta"
	w = do(t, r2, http.MethodPatch, "/api/v1/customers/"+created.ID.String(), customer.UpdateRequest{Name: &newName})
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestList_ReviewerSeesAll(t *testing.T) {
	rSales, db, _ := setupHandler(t, customer.RoleSalesperson)
	require.Equal(t, http.StatusCreated,
		do(t, rSales, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: "X"}).Code)

	// Reviewer router bound to the same DB.
	roleID := lookupRoleID(t, db, customer.RoleReviewer)
	rev := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		rev, "Rev", "rev-"+rev.String()+"@test.local", "h", roleID,
	).Error)
	repo := customer.NewRepository(db)
	h := customer.NewHandler(customer.NewService(repo))
	engine := gin.New()
	v1 := engine.Group("/api/v1")
	v1.Use(func(c *gin.Context) {
		c.Set("auth.currentUser", auth.CurrentUserSummary{ID: rev, Role: customer.RoleReviewer})
		c.Next()
	})
	customer.RegisterRoutes(v1, h)

	w := do(t, engine, http.MethodGet, "/api/v1/customers", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp customer.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.EqualValues(t, 1, resp.Total)
}

func TestCreate_ValidatesName(t *testing.T) {
	r, _, _ := setupHandler(t, customer.RoleSalesperson)
	w := do(t, r, http.MethodPost, "/api/v1/customers", customer.CreateRequest{Name: ""})
	require.Equal(t, http.StatusBadRequest, w.Code)
}
