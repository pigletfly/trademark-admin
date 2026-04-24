// Package migrator applies SQL migrations via golang-migrate.
// Callers provide the migrations as an embed.FS.
package migrator

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrator wraps golang-migrate.
type Migrator struct {
	m *migrate.Migrate
}

// New creates a Migrator using the provided embedded filesystem and Postgres DSN.
// subdir is the path within the FS that contains migration SQL files
// (e.g. "migrations").
func New(migrationsFS fs.FS, subdir, dsn string) (*Migrator, error) {
	src, err := iofs.New(migrationsFS, subdir)
	if err != nil {
		return nil, fmt.Errorf("open migrations fs: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return nil, fmt.Errorf("new migrate: %w", err)
	}
	return &Migrator{m: m}, nil
}

// Up applies all pending migrations. Returns nil if nothing to apply.
func (x *Migrator) Up() error {
	if err := x.m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Down rolls back all migrations.
func (x *Migrator) Down() error {
	if err := x.m.Down(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Steps moves forward (positive) or backward (negative) by n migrations.
func (x *Migrator) Steps(n int) error {
	if err := x.m.Steps(n); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// Version returns the current version and dirty flag.
// Returns (0, false, nil) when no migration has been applied yet.
func (x *Migrator) Version() (uint, bool, error) {
	v, dirty, err := x.m.Version()
	if err == migrate.ErrNilVersion {
		return 0, false, nil
	}
	return v, dirty, err
}

// Close releases the database connection and source.
func (x *Migrator) Close() error {
	srcErr, dbErr := x.m.Close()
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}
