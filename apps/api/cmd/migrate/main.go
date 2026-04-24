// Command migrate applies database migrations manually.
//
// Usage:
//
//	migrate up                    Apply all pending migrations.
//	migrate down                  Revert all migrations.
//	migrate steps <N>             Apply N migrations (negative to roll back).
//	migrate version               Print current version and dirty flag.
package main

import (
	"fmt"
	"os"
	"strconv"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cfg, err := config.Load()
	if err != nil {
		fail("config: %v", err)
	}

	m, err := migrator.New(api.Migrations, "migrations", cfg.DatabaseURL)
	if err != nil {
		fail("migrator: %v", err)
	}
	defer m.Close()

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil {
			fail("up: %v", err)
		}
		fmt.Println("migrations applied")
	case "down":
		if err := m.Down(); err != nil {
			fail("down: %v", err)
		}
		fmt.Println("migrations reverted")
	case "steps":
		if len(os.Args) != 3 {
			usage()
		}
		n, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fail("invalid steps: %v", err)
		}
		if err := m.Steps(n); err != nil {
			fail("steps: %v", err)
		}
		fmt.Printf("stepped %d\n", n)
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			fail("version: %v", err)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate up|down|steps <N>|version")
	os.Exit(2)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", a...)
	os.Exit(1)
}
