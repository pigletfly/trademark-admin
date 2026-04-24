//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func freshDB(t *testing.T) *gorm.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	ctr, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("tm"), postgres.WithUsername("tm"), postgres.WithPassword("tm"),
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
		t.Fatalf("migrator: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	_ = m.Close()

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db
}

func TestFindRoleByCode(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)

	role, err := repo.FindRoleByCode(context.Background(), "admin")
	if err != nil {
		t.Fatalf("FindRoleByCode: %v", err)
	}
	if role.Code != "admin" {
		t.Fatalf("Code = %q", role.Code)
	}
	if role.Name == "" {
		t.Fatalf("Name should be seeded")
	}
}

func TestCreateAndFindUser(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	adminRole, err := repo.FindRoleByCode(ctx, "admin")
	if err != nil {
		t.Fatalf("FindRoleByCode: %v", err)
	}

	u := &auth.User{
		ID:                uuid.New(),
		Name:              "Test Admin",
		Email:             "admin@example.com",
		PasswordHash:      "$argon2id$fake$hash",
		PasswordUpdatedAt: time.Now(),
		RoleID:            adminRole.ID,
		Status:            "active",
	}
	if err := repo.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	byEmail, err := repo.FindUserByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if byEmail.ID != u.ID {
		t.Fatalf("ID mismatch")
	}
	if byEmail.Role.Code != "admin" {
		t.Fatalf("Role.Code = %q, want admin", byEmail.Role.Code)
	}

	byID, err := repo.FindUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindUserByID: %v", err)
	}
	if byID.Email != "admin@example.com" {
		t.Fatalf("Email mismatch")
	}
}

func TestCountUsers(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	n, err := repo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if n != 0 {
		t.Fatalf("fresh db should have 0 users, got %d", n)
	}
}
