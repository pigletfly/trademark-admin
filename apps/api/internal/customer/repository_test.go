package customer_test

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
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

// bootstrap spins up pg, migrates, seeds a salesperson role + user, and
// returns the *gorm.DB together with the owner uuid.
func bootstrap(t *testing.T) (*gorm.DB, uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("custtest"),
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

	// Insert a dummy owner user linked to the seeded salesperson role.
	// Scan into string first: GORM returns UUIDs as text strings over pgx,
	// and [16]byte cannot be scanned from a string directly.
	var roleIDStr string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "salesperson").Scan(&roleIDStr).Error)
	roleID, err := uuid.Parse(roleIDStr)
	require.NoError(t, err)
	owner := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		owner, "Test Owner", "owner-"+owner.String()+"@test.local", "hash", roleID,
	).Error)

	return db, owner
}

func TestRepository_CreateAndGet(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	c := &customer.Customer{Name: "Acme", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), c))
	require.NotEqual(t, uuid.Nil, c.ID)

	got, err := repo.Get(context.Background(), c.ID, nil)
	require.NoError(t, err)
	require.Equal(t, "Acme", got.Name)
}

func TestRepository_Create_DuplicateName(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Acme", CreatedBy: owner}))
	err := repo.Create(context.Background(), &customer.Customer{Name: "Acme", CreatedBy: owner})
	require.ErrorIs(t, err, customer.ErrDuplicateName)
}

func TestRepository_List_OwnerScoped(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	// Create another owner to seed foreign rows.
	var roleIDStr2 string
	require.NoError(t, db.Raw("SELECT id FROM roles WHERE code = ?", "salesperson").Scan(&roleIDStr2).Error)
	roleID2, err := uuid.Parse(roleIDStr2)
	require.NoError(t, err)
	otherOwner := uuid.New()
	require.NoError(t, db.Exec(
		`INSERT INTO users (id, name, email, password_hash, role_id) VALUES (?, ?, ?, ?, ?)`,
		otherOwner, "Another", "other-"+otherOwner.String()+"@test.local", "h", roleID2,
	).Error)

	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "A-mine", CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "B-mine", CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "C-other", CreatedBy: otherOwner}))

	// Scoped to owner: 2 rows.
	res, err := repo.List(context.Background(), customer.ListFilter{OwnerID: &owner})
	require.NoError(t, err)
	require.EqualValues(t, 2, res.Total)
	require.Len(t, res.Items, 2)

	// Unscoped: 3 rows.
	resAll, err := repo.List(context.Background(), customer.ListFilter{})
	require.NoError(t, err)
	require.EqualValues(t, 3, resAll.Total)
}

func TestRepository_List_QueryIlike(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	indFinance := "Finance"
	indHealth := "Healthcare"
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Globex", Industry: &indFinance, CreatedBy: owner}))
	require.NoError(t, repo.Create(context.Background(), &customer.Customer{Name: "Initech", Industry: &indHealth, CreatedBy: owner}))

	res, err := repo.List(context.Background(), customer.ListFilter{Query: "glob"})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	require.Equal(t, "Globex", res.Items[0].Name)

	res, err = repo.List(context.Background(), customer.ListFilter{Query: "HEALTH"})
	require.NoError(t, err)
	require.EqualValues(t, 1, res.Total)
	require.Equal(t, "Initech", res.Items[0].Name)
}

func TestRepository_Update_OwnerGuard(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	c := &customer.Customer{Name: "Acme", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), c))

	// Guarded update by the correct owner.
	newName := "Acme Inc"
	got, err := repo.Update(context.Background(), c.ID, &owner, customer.Patch{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Acme Inc", got.Name)

	// Guarded update by a different owner: not found.
	other := uuid.New()
	_, err = repo.Update(context.Background(), c.ID, &other, customer.Patch{Name: &newName})
	require.ErrorIs(t, err, customer.ErrNotFound)
}

func TestRepository_Update_DuplicateName(t *testing.T) {
	db, owner := bootstrap(t)
	repo := customer.NewRepository(db)

	a := &customer.Customer{Name: "Alpha", CreatedBy: owner}
	b := &customer.Customer{Name: "Bravo", CreatedBy: owner}
	require.NoError(t, repo.Create(context.Background(), a))
	require.NoError(t, repo.Create(context.Background(), b))

	dup := "Alpha"
	_, err := repo.Update(context.Background(), b.ID, nil, customer.Patch{Name: &dup})
	require.ErrorIs(t, err, customer.ErrDuplicateName)
}
