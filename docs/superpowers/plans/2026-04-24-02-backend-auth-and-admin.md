# Backend Auth + Admin User Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Plan 1 的 Go 骨架上搭起认证与权限系统：用户/角色表、密码哈希、JWT + httpOnly cookie、登录/登出/刷新/当前用户接口、角色守卫中间件、CSRF 保护、审计日志中间件，并提供 admin 用户管理接口（`/admin/users*`）和审计日志查询接口（`/admin/audit-logs`）。首次启动自动创建初始 admin。

**Architecture:** `internal/auth/` 模块承载 User/Role 模型、密码哈希、JWT、auth service、HTTP handlers。身份与授权中间件放在同一模块。`internal/platform/audit/` 做跨切 audit log 写入。`cmd/migrate/` 可手动运行迁移；`cmd/server/` 启动时自动迁移 + bootstrap admin。数据库 schema 用 SQL 迁移（golang-migrate），GORM 只做 ORM 查询。

**Tech Stack:** 复用 Plan 1 的 Go 1.25 + Gin v1.10 + GORM v1.25 + PostgreSQL 16。新引入：`github.com/golang-migrate/migrate/v4`（schema migrations）、`github.com/alexedwards/argon2id`（密码哈希）、`github.com/golang-jwt/jwt/v5`（JWT 签发与验证）。

**上下文提示：**
- Plan 1 已经完成：monorepo 建好，`apps/api/` 下已有 `internal/platform/{config,logger,httpx}`、`pkg/database`、`cmd/server/main.go`。`internal/{auth,catalog,customer,pricing,quotation,export}` 是空目录（只有 `.gitkeep`）。
- spec 文件位于 `docs/superpowers/specs/2026-04-24-trademark-quote-platform-mvp-design.md`，§5.1、§9、§11、§12、§13 是本 plan 的需求源。
- `apps/api/go.mod` 的 go 指令是 `go 1.25.0`；Dockerfile 使用 `golang:1.25-alpine`；testcontainers 是 v0.42.0。
- 本 plan 不涉及前端（前端 auth 是 Plan 3）。所有验证通过 curl/Go test 完成。

**在 `feat/plan-2-backend-auth` 分支上执行**（从 main 切出）。

---

## 文件结构（本 plan 结束时新增/修改）

```
apps/api/
├── cmd/
│   ├── server/main.go            (修改：启动时自动 migrate + bootstrap admin + 注册 auth/admin 路由)
│   └── migrate/main.go           (新增：手动迁移 CLI)
├── internal/
│   ├── auth/
│   │   ├── model.go              (User, Role GORM entities)
│   │   ├── password.go           (argon2id hash + verify)
│   │   ├── password_test.go
│   │   ├── jwt.go                (sign/verify access + refresh tokens)
│   │   ├── jwt_test.go
│   │   ├── repository.go         (FindUserByEmail, FindUserByID, FindRoleByCode, Count users, CreateUser, etc.)
│   │   ├── service.go            (Login, Me, Refresh, Bootstrap)
│   │   ├── service_test.go
│   │   ├── middleware.go         (RequireAuth, RequireRole, CSRF)
│   │   ├── middleware_test.go
│   │   ├── handler.go            (HTTP handlers: login/logout/refresh/me)
│   │   ├── handler_test.go
│   │   ├── admin_service.go      (user management: list/create/update/reset-password)
│   │   ├── admin_handler.go
│   │   ├── admin_handler_test.go
│   │   ├── dto.go                (request/response shapes)
│   │   └── router.go             (RegisterRoutes / RegisterAdminRoutes)
│   └── platform/
│       └── audit/
│           ├── model.go          (AuditLog entity)
│           ├── repository.go     (Insert + query)
│           ├── middleware.go     (Gin middleware: write non-GET request to DB)
│           ├── middleware_test.go
│           ├── admin_handler.go  (GET /admin/audit-logs)
│           └── router.go
├── migrations/
│   ├── 000001_init_auth.up.sql       (roles, users, audit_logs tables + seed roles)
│   └── 000001_init_auth.down.sql
├── pkg/
│   └── migrator/
│       ├── migrator.go           (embed migrations/*.sql, provide Up/Down/Force)
│       └── migrator_test.go
├── .env.example                  (新增 BOOTSTRAP_ADMIN_EMAIL/PASSWORD + JWT_ACCESS_SECRET/JWT_REFRESH_SECRET)
docker-compose.yml                (修改：api.environment 新增 BOOTSTRAP_*/JWT_* 变量)
```

---

### Task 1: 搭建迁移工具 `pkg/migrator` + `cmd/migrate`

目标：建立一个可重用的迁移机制，把 SQL 迁移文件 embed 进二进制；提供 `Up(ctx)`、`Down(ctx, steps)`、`Version(ctx)` 三个方法。附带 CLI 工具 `cmd/migrate` 供手动操作。

**Files:**
- Create: `apps/api/pkg/migrator/migrator.go`
- Create: `apps/api/pkg/migrator/migrator_test.go`
- Create: `apps/api/cmd/migrate/main.go`
- Create: `apps/api/migrations/.gitkeep`（保持目录，Task 2 加入实际 SQL）

- [ ] **Step 1: 拉依赖**

Run:
```bash
cd apps/api
go get github.com/golang-migrate/migrate/v4@v4.18.1
go get github.com/golang-migrate/migrate/v4/database/postgres@v4.18.1
go get github.com/golang-migrate/migrate/v4/source/iofs@v4.18.1
cd -
```
Expected: `go.sum` 有这三行。

- [ ] **Step 2: 写一条占位迁移用于测试**

Create `apps/api/migrations/000000_test.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS migration_smoke (id INTEGER PRIMARY KEY);
```

Create `apps/api/migrations/000000_test.down.sql`:
```sql
DROP TABLE IF EXISTS migration_smoke;
```

Create `apps/api/migrations/.gitkeep`（空文件，可以跳过，因为有 SQL 文件了）。

**这两条迁移是临时的，Task 2 开始前要删掉**。写它是为了 Task 1 的测试有东西可以 migrate up/down。

- [ ] **Step 3: 写顶层 migrations_embed.go（供后续测试与 main 共用）**

`//go:embed` 指令只能指向"同目录或子目录"，不能指向 `../migrations`。所以把 embed 放在 `apps/api/` 根（package `api`），由 `cmd/server/`、`cmd/migrate/`、以及测试代码共同引用。

Create `apps/api/migrations_embed.go`:
```go
package api

import "embed"

// Migrations exposes the SQL migrations embedded into the api binary so both
// cmd/server and cmd/migrate can apply the same set without duplicating the
// //go:embed directive.
//
//go:embed all:migrations
var Migrations embed.FS
```

Module 名字已经是 `github.com/pigletfly/trademark-admin/apps/api`，所以 package name 为 `api`。

- [ ] **Step 4: 写失败测试**

Create `apps/api/pkg/migrator/migrator_test.go`:
```go
//go:build integration

package migrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func TestMigratorUpDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("tm"),
		postgres.WithUsername("tm"),
		postgres.WithPassword("tm"),
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
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	if err := m.Up(); err != nil {
		t.Fatalf("Up: %v", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version == 0 {
		t.Fatalf("expected non-zero version after Up, got %d (dirty=%v)", version, dirty)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("Down: %v", err)
	}
}
```

- [ ] **Step 5: 跑测试看失败**

```bash
cd apps/api
go test -tags=integration ./pkg/migrator/... -v
cd -
```
Expected: `undefined: migrator.New`。

- [ ] **Step 6: 写 migrator 实现**

Create `apps/api/pkg/migrator/migrator.go`:
```go
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
```

- [ ] **Step 7: 跑测试看通过**

```bash
cd apps/api
go test -tags=integration ./pkg/migrator/... -v
cd -
```
Expected: `PASS`（会看到 `000000_test` 被 Up 然后 Down）。

- [ ] **Step 8: 写 cmd/migrate/main.go**

Create `apps/api/cmd/migrate/main.go`:
```go
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
```

- [ ] **Step 9: 编译整个后端**

```bash
cd apps/api
go build ./...
cd -
```
Expected: 无输出。

- [ ] **Step 10: 删除临时迁移 + 提交**

Run:
```bash
rm apps/api/migrations/000000_test.up.sql apps/api/migrations/000000_test.down.sql
```

把临时 `000000_test*.sql` 删掉；下一步 Task 2 会加入真正的 0001 迁移。

测试到此应该会因为"migrations 目录为空"失败——没关系，Task 2 会加回迁移后再次跑。但 build 必须通过。

```bash
cd apps/api
go build ./...
cd -
```

提交（**先不提交迁移被删这件事，把它和 Task 2 合并提交**；本 task 只提交 migrator + cmd/migrate + embed 基础设施）：
```bash
git add apps/api/pkg/migrator/ apps/api/cmd/migrate/ apps/api/migrations_embed.go apps/api/go.mod apps/api/go.sum
git commit -m "feat(api): add embedded migration runner (pkg/migrator + cmd/migrate)"
```

---

### Task 2: 写第一条迁移：roles、users、audit_logs 表 + roles seed

目标：建立身份系统的三张表。

**Files:**
- Create: `apps/api/migrations/000001_init_auth.up.sql`
- Create: `apps/api/migrations/000001_init_auth.down.sql`

- [ ] **Step 1: 写 up 迁移**

Create `apps/api/migrations/000001_init_auth.up.sql`:
```sql
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS roles (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code         TEXT UNIQUE NOT NULL,
  name         TEXT NOT NULL,
  description  TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the three MVP roles. Codes are referenced from Go middleware.
INSERT INTO roles (code, name, description) VALUES
  ('salesperson', '业务员',   '录入客户需求、预览和下载报价、查看自建历史'),
  ('reviewer',    '国际部商务', '审核报价、维护成本模板、管理供应商'),
  ('admin',       '系统管理员', '维护字典、管理用户、查看审计日志')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name                  TEXT NOT NULL,
  phone                 TEXT,
  email                 CITEXT UNIQUE NOT NULL,
  password_hash         TEXT NOT NULL,
  password_updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  role_id               UUID NOT NULL REFERENCES roles(id),
  status                TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'disabled')),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role_id);

CREATE TABLE IF NOT EXISTS audit_logs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID REFERENCES users(id),
  action          TEXT NOT NULL,
  resource_type   TEXT NOT NULL,
  resource_id     TEXT NOT NULL,
  changes_json    JSONB,
  ip              INET,
  user_agent      TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_user_time ON audit_logs(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_type, resource_id);
```

- [ ] **Step 2: 写 down 迁移**

Create `apps/api/migrations/000001_init_auth.down.sql`:
```sql
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
-- keep extensions alone; they may be in use by other databases / installations
```

- [ ] **Step 3: 跑迁移测试**

```bash
cd apps/api
go test -tags=integration ./pkg/migrator/... -v
cd -
```
Expected: PASS；Up 会把版本推到 1，Down 回到 0。

- [ ] **Step 4: 手动在本地 postgres 上跑一次迁移确认可用**

```bash
docker compose up -d postgres
sleep 5
cd apps/api
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable go run ./cmd/migrate up
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable go run ./cmd/migrate version
cd -
docker compose exec postgres psql -U trademark -d trademark -c "SELECT code, name FROM roles ORDER BY code;"
```
Expected：
- `migrations applied`
- `version=1 dirty=false`
- 三行：admin / reviewer / salesperson

把数据库恢复干净：
```bash
docker compose down -v
```

- [ ] **Step 5: 提交**

```bash
git add apps/api/migrations/
git commit -m "feat(api): migration 0001 initialises auth schema and seeds roles"
```

---

### Task 3: 密码哈希模块 `internal/auth/password`

**Files:**
- Create: `apps/api/internal/auth/password.go`
- Create: `apps/api/internal/auth/password_test.go`

- [ ] **Step 1: 拉依赖**

```bash
cd apps/api
go get github.com/alexedwards/argon2id@v1.0.0
cd -
```

- [ ] **Step 2: 写失败测试**

Create `apps/api/internal/auth/password_test.go`:
```go
package auth_test

import (
	"strings"
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestHashPassword_producesPHCEncodedHash(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected prefix: %q", hash)
	}
}

func TestVerifyPassword_matchAndMismatch(t *testing.T) {
	hash, err := auth.HashPassword("super-secret-1")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := auth.VerifyPassword("super-secret-1", hash)
	if err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	ok, err = auth.VerifyPassword("super-secret-2", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned unexpected error on mismatch: %v", err)
	}
	if ok {
		t.Fatalf("expected mismatch, got ok=true")
	}
}

func TestHashPassword_uniqueSaltPerCall(t *testing.T) {
	h1, _ := auth.HashPassword("pw")
	h2, _ := auth.HashPassword("pw")
	if h1 == h2 {
		t.Fatalf("two hashes of same password collided; salt not random? h=%q", h1)
	}
}
```

- [ ] **Step 3: 跑测试看失败**

```bash
cd apps/api
go test ./internal/auth/... -v
cd -
```
Expected: `undefined: auth.HashPassword / auth.VerifyPassword`。

- [ ] **Step 4: 写实现**

Create `apps/api/internal/auth/password.go`:
```go
// Package auth provides authentication primitives and middleware.
//
// This file exposes password hashing/verification using argon2id. Parameters
// match the spec (§12): memory=64MiB, iterations=3, parallelism=2.
package auth

import "github.com/alexedwards/argon2id"

var passwordParams = &argon2id.Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword returns a PHC-encoded argon2id hash of the password. The salt is
// generated per call, so two hashes of the same password differ.
func HashPassword(plain string) (string, error) {
	return argon2id.CreateHash(plain, passwordParams)
}

// VerifyPassword reports whether plain matches the stored PHC-encoded hash.
// It never returns a nil-error-with-ok=false panic: a mismatch is a clean
// (false, nil).
func VerifyPassword(plain, encoded string) (bool, error) {
	return argon2id.ComparePasswordAndHash(plain, encoded)
}
```

- [ ] **Step 5: 跑测试看通过**

```bash
cd apps/api
go test ./internal/auth/... -v
cd -
```
Expected: 3 条测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/auth/password.go apps/api/internal/auth/password_test.go apps/api/go.mod apps/api/go.sum
git commit -m "feat(auth): add argon2id password hashing helpers"
```

---

### Task 4: JWT 签发与验证 `internal/auth/jwt`

**Files:**
- Create: `apps/api/internal/auth/jwt.go`
- Create: `apps/api/internal/auth/jwt_test.go`
- Modify: `apps/api/internal/platform/config/config.go` + test（加 `JWTAccessSecret` 等字段）

- [ ] **Step 1: 拉依赖**

```bash
cd apps/api
go get github.com/golang-jwt/jwt/v5@v5.2.1
cd -
```

- [ ] **Step 2: 扩展 Config**

修改 `apps/api/internal/platform/config/config.go`：
- 在 `Config` struct 加 5 个字段：`JWTAccessSecret string`、`JWTRefreshSecret string`、`JWTAccessTTL time.Duration`、`JWTRefreshTTL time.Duration`、`CookieSecure bool`
- 在 `Load()` 从 env 读取；`JWTAccessSecret`/`JWTRefreshSecret` 必填（若空返回错误）；`JWTAccessTTL` 默认 `15m`、`JWTRefreshTTL` 默认 `168h`（7 天）；`CookieSecure` 默认 false

新的 `Load()` 返回 Config 指针版本（**保持向后兼容**：把错误类型保持 `errors.New`）。

解析 duration 用 `time.ParseDuration(getenvDefault("JWT_ACCESS_TTL", "15m"))`，错误返回包装错误。

完整新版 `apps/api/internal/platform/config/config.go`:
```go
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL    string
	HTTPListenAddr string
	LogLevel       string
	AppEnv         string

	JWTAccessSecret  string
	JWTRefreshSecret string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	CookieSecure     bool
}

// Load reads configuration from environment variables and applies defaults.
// DATABASE_URL, JWT_ACCESS_SECRET, and JWT_REFRESH_SECRET are required.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		HTTPListenAddr:   getenvDefault("HTTP_LISTEN_ADDR", ":8080"),
		LogLevel:         getenvDefault("LOG_LEVEL", "info"),
		AppEnv:           getenvDefault("APP_ENV", "development"),
		JWTAccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTAccessSecret == "" {
		return nil, errors.New("JWT_ACCESS_SECRET is required")
	}
	if cfg.JWTRefreshSecret == "" {
		return nil, errors.New("JWT_REFRESH_SECRET is required")
	}

	accessTTL, err := parseDuration("JWT_ACCESS_TTL", "15m")
	if err != nil {
		return nil, err
	}
	refreshTTL, err := parseDuration("JWT_REFRESH_TTL", "168h")
	if err != nil {
		return nil, err
	}
	cfg.JWTAccessTTL = accessTTL
	cfg.JWTRefreshTTL = refreshTTL

	secure, _ := strconv.ParseBool(getenvDefault("COOKIE_SECURE", "false"))
	cfg.CookieSecure = secure

	return cfg, nil
}

func parseDuration(key, fallback string) (time.Duration, error) {
	val := getenvDefault(key, fallback)
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid duration (%s): %w", key, val, err)
	}
	return d, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: 更新 config 测试**

修改 `apps/api/internal/platform/config/config_test.go`：
- `TestLoad_defaults` 在已有 setenv 基础上多 set `JWT_ACCESS_SECRET=dev`、`JWT_REFRESH_SECRET=dev2`；期望 `cfg.JWTAccessTTL == 15*time.Minute` 和 `cfg.JWTRefreshTTL == 168*time.Hour`
- `TestLoad_missingDatabaseURL` 保留
- 新增 `TestLoad_missingJWTSecrets`：分别把 `JWT_ACCESS_SECRET` 和 `JWT_REFRESH_SECRET` 置空，期望错误

完整新版 `config_test.go`:
```go
package config_test

import (
	"testing"
	"time"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
)

func setValidBaseEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@localhost:5432/c?sslmode=disable")
	t.Setenv("JWT_ACCESS_SECRET", "dev-access")
	t.Setenv("JWT_REFRESH_SECRET", "dev-refresh")
}

func TestLoad_defaults(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("HTTP_LISTEN_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("JWT_ACCESS_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")
	t.Setenv("COOKIE_SECURE", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPListenAddr != ":8080" {
		t.Errorf("HTTPListenAddr = %q, want :8080", cfg.HTTPListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Errorf("JWTAccessTTL = %v, want 15m", cfg.JWTAccessTTL)
	}
	if cfg.JWTRefreshTTL != 168*time.Hour {
		t.Errorf("JWTRefreshTTL = %v, want 168h", cfg.JWTRefreshTTL)
	}
	if cfg.CookieSecure {
		t.Errorf("CookieSecure should default to false")
	}
}

func TestLoad_missingDatabaseURL(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("DATABASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_missingJWTAccessSecret(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_ACCESS_SECRET", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_missingJWTRefreshSecret(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_REFRESH_SECRET", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoad_invalidDuration(t *testing.T) {
	setValidBaseEnv(t)
	t.Setenv("JWT_ACCESS_TTL", "not-a-duration")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error")
	}
}
```

- [ ] **Step 4: 跑测试（config）**

```bash
cd apps/api
go test ./internal/platform/config/... -v
cd -
```
Expected: 5 条 PASS。

- [ ] **Step 5: 写 jwt 失败测试**

Create `apps/api/internal/auth/jwt_test.go`:
```go
package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	secret := []byte("access-secret")
	userID := uuid.New()
	role := "admin"

	token, err := auth.IssueAccessToken(secret, userID, role, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := auth.ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.TokenType != "access" {
		t.Errorf("TokenType = %q, want access", claims.TokenType)
	}
}

func TestAccessTokenExpires(t *testing.T) {
	secret := []byte("access-secret")
	userID := uuid.New()

	token, err := auth.IssueAccessToken(secret, userID, "admin", -1*time.Second)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	if _, err := auth.ParseAccessToken(secret, token); err == nil {
		t.Fatalf("expected expiration error")
	}
}

func TestRefreshTokenSeparateSecret(t *testing.T) {
	access := []byte("access-secret")
	refresh := []byte("refresh-secret")
	userID := uuid.New()

	token, err := auth.IssueRefreshToken(refresh, userID, time.Hour)
	if err != nil {
		t.Fatalf("IssueRefreshToken: %v", err)
	}

	// Wrong secret (access secret) must not parse a refresh token.
	if _, err := auth.ParseRefreshToken(access, token); err == nil {
		t.Fatalf("expected signature error")
	}
	claims, err := auth.ParseRefreshToken(refresh, token)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %q, want refresh", claims.TokenType)
	}
}

func TestAccessTokenTypeNotAcceptedAsRefresh(t *testing.T) {
	secret := []byte("same-secret-wrong-usage")
	userID := uuid.New()
	accessToken, _ := auth.IssueAccessToken(secret, userID, "admin", time.Hour)
	if _, err := auth.ParseRefreshToken(secret, accessToken); err == nil {
		t.Fatalf("expected type mismatch error")
	}
}
```

- [ ] **Step 6: 跑测试看失败**

```bash
cd apps/api
go get github.com/google/uuid@v1.6.0
go test ./internal/auth/... -v
cd -
```
Expected: `undefined: auth.IssueAccessToken / ParseAccessToken / IssueRefreshToken / ParseRefreshToken`。

- [ ] **Step 7: 写实现**

Create `apps/api/internal/auth/jwt.go`:
```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType discriminates access vs refresh tokens so a stolen access token
// cannot be used to refresh, and vice versa.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// Claims is the JWT payload used by both token types.
type Claims struct {
	UserID    uuid.UUID `json:"-"`
	Role      string    `json:"role,omitempty"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs a short-lived access token carrying the user's role.
func IssueAccessToken(secret []byte, userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	return issue(secret, userID, role, TokenTypeAccess, ttl)
}

// IssueRefreshToken signs a long-lived refresh token without role info.
func IssueRefreshToken(secret []byte, userID uuid.UUID, ttl time.Duration) (string, error) {
	return issue(secret, userID, "", TokenTypeRefresh, ttl)
}

func issue(secret []byte, userID uuid.UUID, role string, typ TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// ParseAccessToken verifies an access token's signature and type.
func ParseAccessToken(secret []byte, tokenString string) (*Claims, error) {
	return parse(secret, tokenString, TokenTypeAccess)
}

// ParseRefreshToken verifies a refresh token's signature and type.
func ParseRefreshToken(secret []byte, tokenString string) (*Claims, error) {
	return parse(secret, tokenString, TokenTypeRefresh)
}

func parse(secret []byte, tokenString string, expected TokenType) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != expected {
		return nil, fmt.Errorf("token type %q, want %q", claims.TokenType, expected)
	}
	// Reconstruct UserID from Subject so the uuid.UUID field is populated.
	if id, err := uuid.Parse(claims.Subject); err == nil {
		claims.UserID = id
	}
	return claims, nil
}
```

- [ ] **Step 8: 跑测试看通过**

```bash
cd apps/api
go test ./internal/auth/... -v
cd -
```
Expected: 4 条 jwt 测试 + 3 条 password 测试全通过。

- [ ] **Step 9: 提交**

```bash
git add apps/api/internal/auth/jwt.go apps/api/internal/auth/jwt_test.go apps/api/internal/platform/config/ apps/api/go.mod apps/api/go.sum
git commit -m "feat(auth): add JWT issue/parse + extend config with JWT secrets/TTLs"
```

---

### Task 5: GORM 模型 + Repository `internal/auth/model.go + repository.go`

**Files:**
- Create: `apps/api/internal/auth/model.go`
- Create: `apps/api/internal/auth/repository.go`
- Create: `apps/api/internal/auth/repository_test.go`

- [ ] **Step 1: 写 model.go**

Create `apps/api/internal/auth/model.go`:
```go
package auth

import (
	"time"

	"github.com/google/uuid"
)

// Role is a platform role (salesperson, reviewer, admin).
type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code        string    `gorm:"uniqueIndex;not null"`
	Name        string    `gorm:"not null"`
	Description string
	CreatedAt   time.Time
}

// User represents a platform user with exactly one role.
type User struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name              string    `gorm:"not null"`
	Phone             string
	Email             string    `gorm:"type:citext;uniqueIndex;not null"`
	PasswordHash      string    `gorm:"not null"`
	PasswordUpdatedAt time.Time `gorm:"not null"`
	RoleID            uuid.UUID `gorm:"type:uuid;not null;index"`
	Role              Role      `gorm:"foreignKey:RoleID;references:ID"`
	Status            string    `gorm:"not null;default:active"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TableName overrides GORM pluralization for clarity.
func (Role) TableName() string { return "roles" }
func (User) TableName() string { return "users" }
```

- [ ] **Step 2: 写 repository 失败测试**

Create `apps/api/internal/auth/repository_test.go`:
```go
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
```

- [ ] **Step 3: 跑测试看失败**

```bash
cd apps/api
go test -tags=integration ./internal/auth/... -v
cd -
```
Expected: `undefined: auth.NewRepository` 等。

- [ ] **Step 4: 写 repository.go**

Create `apps/api/internal/auth/repository.go`:
```go
package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrNotFound indicates the queried entity does not exist.
	ErrNotFound = errors.New("auth: not found")
)

// Repository encapsulates user/role persistence.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository bound to the given GORM handle.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// FindRoleByCode returns the role with the given code or ErrNotFound.
func (r *Repository) FindRoleByCode(ctx context.Context, code string) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindUserByEmail returns the user with the given email (with Role preloaded).
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role").
		Where("email = ?", email).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByID returns the user with the given id (with Role preloaded).
func (r *Repository) FindUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).
		Preload("Role").
		First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUser persists a new user. Fills ID if zero.
func (r *Repository) CreateUser(ctx context.Context, u *User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(u).Error
}

// CountUsers returns the total number of users (used for bootstrap decisions).
func (r *Repository) CountUsers(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&User{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// UpdateUser persists changes to an existing user. Uses full-struct update
// (columns listed in fields). Empty fields slice updates all non-zero columns.
func (r *Repository) UpdateUser(ctx context.Context, u *User, fields ...string) error {
	q := r.db.WithContext(ctx).Model(u)
	if len(fields) > 0 {
		q = q.Select(fields)
	}
	return q.Updates(u).Error
}

// ListUsers returns users filtered by optional email prefix and role code.
// page is 1-based.
func (r *Repository) ListUsers(ctx context.Context, q string, roleCode string, page, pageSize int) ([]User, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := r.db.WithContext(ctx).Model(&User{}).Preload("Role")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("email ILIKE ? OR name ILIKE ?", like, like)
	}
	if roleCode != "" {
		query = query.Joins("JOIN roles ON roles.id = users.role_id").
			Where("roles.code = ?", roleCode)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []User
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("users.created_at DESC").Find(&out).Error
	return out, total, err
}
```

- [ ] **Step 5: 跑测试看通过**

```bash
cd apps/api
go test -tags=integration ./internal/auth/... -v
cd -
```
Expected: 3 条 PASS。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/auth/model.go apps/api/internal/auth/repository.go apps/api/internal/auth/repository_test.go
git commit -m "feat(auth): add User/Role models + repository with integration tests"
```

---

### Task 6: Auth service `Login / Me / Refresh / Bootstrap` + 错误类型

**Files:**
- Create: `apps/api/internal/auth/service.go`
- Create: `apps/api/internal/auth/service_test.go`
- Create: `apps/api/internal/auth/dto.go`（空占位，handler 才会实际填）
- Modify: `apps/api/internal/auth/repository.go`（如有方法补足）

- [ ] **Step 1: 写失败测试**

Create `apps/api/internal/auth/service_test.go`:
```go
//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func TestService_BootstrapAdmin(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:             repo,
		AccessSecret:     []byte("a"),
		RefreshSecret:    []byte("r"),
		AccessTTL:        5 * time.Minute,
		RefreshTTL:       time.Hour,
	})

	if err := svc.Bootstrap(context.Background(), "root@example.com", "initial-pass-123", "Root Admin"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	n, _ := repo.CountUsers(context.Background())
	if n != 1 {
		t.Fatalf("expected 1 user, got %d", n)
	}
	// Running Bootstrap again is a no-op when a user already exists.
	if err := svc.Bootstrap(context.Background(), "root@example.com", "initial-pass-123", "Root Admin"); err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}
	n, _ = repo.CountUsers(context.Background())
	if n != 1 {
		t.Fatalf("Bootstrap should be idempotent, got %d users", n)
	}
}

func TestService_LoginSuccess(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()

	if err := svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	result, err := svc.Login(ctx, "root@example.com", "pw-abcdefg-1234")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("tokens empty")
	}
	if result.User.Email != "root@example.com" {
		t.Fatalf("user email mismatch")
	}
	if result.User.Role.Code != "admin" {
		t.Fatalf("bootstrapped user should have admin role, got %q", result.User.Role.Code)
	}
}

func TestService_LoginWrongPassword(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()
	_ = svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root")

	_, err := svc.Login(ctx, "root@example.com", "wrong-password")
	if err == nil {
		t.Fatalf("expected login error")
	}
}

func TestService_Refresh(t *testing.T) {
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	ctx := context.Background()
	_ = svc.Bootstrap(ctx, "root@example.com", "pw-abcdefg-1234", "Root")

	login, err := svc.Login(ctx, "root@example.com", "pw-abcdefg-1234")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	refreshed, err := svc.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Fatalf("Refresh must return a new access token")
	}
}
```

- [ ] **Step 2: 跑测试看失败**

```bash
cd apps/api
go test -tags=integration ./internal/auth/... -v
cd -
```
Expected: `undefined: auth.NewService / ServiceConfig / Bootstrap / Login / Refresh`。

- [ ] **Step 3: 写 service.go**

Create `apps/api/internal/auth/service.go`:
```go
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LoginResult bundles tokens and the user profile returned to clients.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *User
}

// RefreshResult is returned by Service.Refresh.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string // may be rotated; MVP reuses same for simplicity
	User         *User
}

// ServiceConfig bundles the dependencies Service needs.
type ServiceConfig struct {
	Repo          *Repository
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

// Service offers login, refresh, me, and bootstrap use-cases.
type Service struct {
	cfg ServiceConfig
}

// NewService constructs a Service.
func NewService(cfg ServiceConfig) *Service { return &Service{cfg: cfg} }

// ErrInvalidCredentials is returned when email or password does not match.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrUserDisabled is returned when the account exists but is not active.
var ErrUserDisabled = errors.New("auth: user disabled")

// Bootstrap creates the very first admin user if the users table is empty.
// It is idempotent: on a non-empty table it returns nil.
func (s *Service) Bootstrap(ctx context.Context, email, password, displayName string) error {
	n, err := s.cfg.Repo.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}
	role, err := s.cfg.Repo.FindRoleByCode(ctx, "admin")
	if err != nil {
		return fmt.Errorf("find admin role: %w", err)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now()
	u := &User{
		ID:                uuid.New(),
		Name:              displayName,
		Email:             email,
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		RoleID:            role.ID,
		Status:            "active",
	}
	return s.cfg.Repo.CreateUser(ctx, u)
}

// Login validates credentials and issues tokens.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.cfg.Repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrUserDisabled
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}
	access, err := IssueAccessToken(s.cfg.AccessSecret, user.ID, user.Role.Code, s.cfg.AccessTTL)
	if err != nil {
		return nil, fmt.Errorf("issue access: %w", err)
	}
	refresh, err := IssueRefreshToken(s.cfg.RefreshSecret, user.ID, s.cfg.RefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("issue refresh: %w", err)
	}
	return &LoginResult{AccessToken: access, RefreshToken: refresh, User: user}, nil
}

// Refresh validates the refresh token and issues a new access token.
// MVP does not rotate refresh tokens (returns the same one) — acceptable because
// refresh TTL is short-ish (7 days) and the cookie is httpOnly + SameSite=Lax.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*RefreshResult, error) {
	claims, err := ParseRefreshToken(s.cfg.RefreshSecret, refreshToken)
	if err != nil {
		return nil, err
	}
	user, err := s.cfg.Repo.FindUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrUserDisabled
	}
	access, err := IssueAccessToken(s.cfg.AccessSecret, user.ID, user.Role.Code, s.cfg.AccessTTL)
	if err != nil {
		return nil, err
	}
	return &RefreshResult{AccessToken: access, RefreshToken: refreshToken, User: user}, nil
}

// Me looks up a user by ID (used by /auth/me handler).
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.cfg.Repo.FindUserByID(ctx, userID)
}
```

- [ ] **Step 4: 跑测试看通过**

```bash
cd apps/api
go test -tags=integration ./internal/auth/... -v
cd -
```
Expected: 4 条 service 测试 + 3 条 repository 测试全通过。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/auth/service.go apps/api/internal/auth/service_test.go
git commit -m "feat(auth): add Login/Refresh/Bootstrap service with integration tests"
```

---

### Task 7: 中间件合集 `RequireAuth / RequireRole / CSRF`

**Files:**
- Create: `apps/api/internal/auth/middleware.go`
- Create: `apps/api/internal/auth/middleware_test.go`

- [ ] **Step 1: 写失败测试**

Create `apps/api/internal/auth/middleware_test.go`:
```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func setupRouter(accessSecret []byte, requireRoles ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/")
	protected.Use(auth.RequireAuth(accessSecret))
	if len(requireRoles) > 0 {
		protected.Use(auth.RequireRole(requireRoles...))
	}
	protected.GET("/whoami", func(c *gin.Context) {
		user := auth.CurrentUser(c)
		c.JSON(http.StatusOK, gin.H{"user_id": user.ID, "role": user.Role})
	})
	return r
}

func TestRequireAuth_missingCookie(t *testing.T) {
	r := setupRouter([]byte("s"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}

func TestRequireAuth_validCookie(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, err := auth.IssueAccessToken(secret, uid, "salesperson", time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	r := setupRouter(secret)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestRequireRole_forbidden(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, _ := auth.IssueAccessToken(secret, uid, "salesperson", time.Minute)

	r := setupRouter(secret, "admin")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", w.Code)
	}
}

func TestRequireRole_allowed(t *testing.T) {
	secret := []byte("sekret")
	uid := uuid.New()
	token, _ := auth.IssueAccessToken(secret, uid, "admin", time.Minute)

	r := setupRouter(secret, "admin", "reviewer")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestCSRF_blocksMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.POST("/do", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/do", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieCSRFToken, Value: "abc"})
	// Header missing
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", w.Code)
	}
}

func TestCSRF_passesWhenMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.POST("/do", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/do", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieCSRFToken, Value: "abc"})
	req.Header.Set("X-CSRF-Token", "abc")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
}

func TestCSRF_ignoresSafeMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(auth.CSRF())
	r.GET("/read", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("CSRF must pass through GET; got %d", w.Code)
	}
}
```

- [ ] **Step 2: 跑测试看失败**

```bash
cd apps/api
go test ./internal/auth/... -v
cd -
```
Expected: 找不到 `RequireAuth / RequireRole / CSRF / CurrentUser / CookieAccessToken / CookieCSRFToken` 等符号。

- [ ] **Step 3: 写实现**

Create `apps/api/internal/auth/middleware.go`:
```go
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Cookie names. Exported so handlers and frontend share them.
const (
	CookieAccessToken  = "tm_access_token"
	CookieRefreshToken = "tm_refresh_token"
	CookieCSRFToken    = "tm_csrf_token"
)

// CurrentUserCtx is the Gin context key storing the authenticated user summary.
const currentUserKey = "auth.currentUser"

// CurrentUserSummary is what RequireAuth puts into the Gin context. Handlers
// should use CurrentUser to fetch it.
type CurrentUserSummary struct {
	ID   uuid.UUID
	Role string // role code
}

// CurrentUser returns the authenticated user summary from the Gin context,
// or a zero value if no RequireAuth middleware ran.
func CurrentUser(c *gin.Context) CurrentUserSummary {
	v, ok := c.Get(currentUserKey)
	if !ok {
		return CurrentUserSummary{}
	}
	return v.(CurrentUserSummary)
}

// RequireAuth verifies the access token cookie and injects the user summary.
func RequireAuth(accessSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(CookieAccessToken)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "ERR_UNAUTHORIZED",
				"message": "authentication required",
			})
			return
		}
		claims, err := ParseAccessToken(accessSecret, cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "ERR_UNAUTHORIZED",
				"message": "invalid or expired token",
			})
			return
		}
		c.Set(currentUserKey, CurrentUserSummary{ID: claims.UserID, Role: claims.Role})
		c.Next()
	}
}

// RequireRole checks that the authenticated user belongs to one of the
// allowed roles. RequireAuth must precede it.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if _, ok := allowed[user.Role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "ERR_FORBIDDEN",
				"message": "role not permitted",
			})
			return
		}
		c.Next()
	}
}

// CSRF enforces double-submit token validation for non-safe HTTP methods.
// Cookie `tm_csrf_token` (not httpOnly) must equal header `X-CSRF-Token`.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		cookie, err := c.Cookie(CookieCSRFToken)
		header := c.GetHeader("X-CSRF-Token")
		if err != nil || cookie == "" || header == "" || cookie != header {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "ERR_CSRF",
				"message": "CSRF token missing or mismatched",
			})
			return
		}
		c.Next()
	}
}

func isSafeMethod(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
```

- [ ] **Step 4: 跑测试看通过**

```bash
cd apps/api
go test ./internal/auth/... -v
cd -
```
Expected: 7 条 middleware 测试 + 3 条 password + 4 条 jwt 全通过；integration 测试也要能编译（不跑，因为无 -tags）。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/auth/middleware.go apps/api/internal/auth/middleware_test.go
git commit -m "feat(auth): add RequireAuth/RequireRole/CSRF middleware"
```

---

### Task 8: Auth HTTP handlers + router `handler.go / router.go / dto.go`

**Files:**
- Create: `apps/api/internal/auth/dto.go`
- Create: `apps/api/internal/auth/handler.go`
- Create: `apps/api/internal/auth/handler_test.go`
- Create: `apps/api/internal/auth/router.go`

- [ ] **Step 1: 写 dto.go**

Create `apps/api/internal/auth/dto.go`:
```go
package auth

import "github.com/google/uuid"

// LoginRequest models the POST /auth/login body.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=1"`
}

// UserResponse is the slimmed-down representation of a user returned to clients.
type UserResponse struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Email  string    `json:"email"`
	Phone  string    `json:"phone,omitempty"`
	Role   string    `json:"role"`   // role code
	Status string    `json:"status"`
}

// ToResponse converts a User into its API-facing shape.
func ToResponse(u *User) UserResponse {
	return UserResponse{
		ID:     u.ID,
		Name:   u.Name,
		Email:  u.Email,
		Phone:  u.Phone,
		Role:   u.Role.Code,
		Status: u.Status,
	}
}
```

- [ ] **Step 2: 写 router.go**

Create `apps/api/internal/auth/router.go`:
```go
package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the /auth/* routes. protect is the authenticated group.
func RegisterRoutes(public *gin.RouterGroup, authenticated *gin.RouterGroup, h *Handler) {
	public.POST("/auth/login", h.Login)
	public.POST("/auth/refresh", h.Refresh)

	authenticated.POST("/auth/logout", h.Logout)
	authenticated.GET("/auth/me", h.Me)
}
```

- [ ] **Step 3: 写 handler.go**

Create `apps/api/internal/auth/handler.go`:
```go
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler holds auth-related HTTP handlers.
type Handler struct {
	svc          *Service
	cookieSecure bool
	cookieDomain string // empty = host-only
	accessTTL    time.Duration
	refreshTTL   time.Duration
}

// HandlerConfig bundles non-service dependencies.
type HandlerConfig struct {
	Service      *Service
	CookieSecure bool
	CookieDomain string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

// NewHandler constructs a Handler.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		svc:          cfg.Service,
		cookieSecure: cfg.CookieSecure,
		cookieDomain: cfg.CookieDomain,
		accessTTL:    cfg.AccessTTL,
		refreshTTL:   cfg.RefreshTTL,
	}
}

// Login handles POST /auth/login.
func (h *Handler) Login(c *gin.Context) {
	var body LoginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	result, err := h.svc.Login(c.Request.Context(), body.Email, body.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_INVALID_CREDENTIALS", "message": "email or password incorrect"})
			return
		}
		if errors.Is(err, ErrUserDisabled) {
			c.JSON(http.StatusForbidden, gin.H{"code": "ERR_USER_DISABLED", "message": "account disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}

	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(result.User)})
}

// Refresh handles POST /auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	rt, err := c.Cookie(CookieRefreshToken)
	if err != nil || rt == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED", "message": "refresh token missing"})
		return
	}
	result, err := h.svc.Refresh(c.Request.Context(), rt)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "ERR_UNAUTHORIZED", "message": err.Error()})
		return
	}
	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(result.User)})
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(c *gin.Context) {
	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

// Me handles GET /auth/me.
func (h *Handler) Me(c *gin.Context) {
	u := CurrentUser(c)
	user, err := h.svc.Me(c.Request.Context(), u.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(user)})
}

func (h *Handler) setAuthCookies(c *gin.Context, access, refresh string) {
	h.setCookie(c, CookieAccessToken, access, int(h.accessTTL.Seconds()), true)
	h.setCookie(c, CookieRefreshToken, refresh, int(h.refreshTTL.Seconds()), true)
	// CSRF token is NOT httpOnly: the JS client must read it and echo via header.
	csrf, _ := randomToken(24)
	h.setCookie(c, CookieCSRFToken, csrf, int(h.refreshTTL.Seconds()), false)
}

func (h *Handler) clearAuthCookies(c *gin.Context) {
	h.setCookie(c, CookieAccessToken, "", -1, true)
	h.setCookie(c, CookieRefreshToken, "", -1, true)
	h.setCookie(c, CookieCSRFToken, "", -1, false)
}

func (h *Handler) setCookie(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	// SameSite=Lax so same-site navigations send the cookie but cross-site POSTs do not.
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, "/", h.cookieDomain, h.cookieSecure, httpOnly)
}

// randomToken returns a URL-safe random string of approx `n` random bytes.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 4: 写 handler 集成测试**

Create `apps/api/internal/auth/handler_test.go`:
```go
//go:build integration

package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func buildRouter(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo:          repo,
		AccessSecret:  []byte("a"),
		RefreshSecret: []byte("r"),
		AccessTTL:     5 * time.Minute,
		RefreshTTL:    time.Hour,
	})
	h := auth.NewHandler(auth.HandlerConfig{
		Service:      svc,
		CookieSecure: false,
		AccessTTL:    5 * time.Minute,
		RefreshTTL:   time.Hour,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	public := r.Group("/api/v1")
	authed := r.Group("/api/v1")
	authed.Use(auth.RequireAuth([]byte("a")))
	auth.RegisterRoutes(public, authed, h)
	return r, svc
}

func TestLoginHandler_setsCookiesAndMe(t *testing.T) {
	r, svc := buildRouter(t)

	if err := svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin"); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	body, _ := json.Marshal(auth.LoginRequest{Email: "admin@example.com", Password: "pw-abcdefg-1234"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d, body=%s", w.Code, w.Body.String())
	}
	resp := w.Result()
	var accessCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieAccessToken {
			accessCookie = c
		}
	}
	if accessCookie == nil || accessCookie.Value == "" {
		t.Fatalf("access cookie missing; got=%v", resp.Cookies())
	}

	// Hit /me with the access cookie.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req2.AddCookie(accessCookie)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("me code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var meBody struct {
		User auth.UserResponse `json:"user"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if meBody.User.Email != "admin@example.com" {
		t.Fatalf("email = %q", meBody.User.Email)
	}
}
```

**注意**：`t.Context()` 是 Go 1.24+ 新增 API；go.mod 声明 go 1.25，本地是 1.26.1，支持。

- [ ] **Step 5: 跑全部测试（non-integration + integration）**

```bash
cd apps/api
go test ./...
go test -tags=integration ./...
cd -
```
Expected: 全绿。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/auth/dto.go apps/api/internal/auth/handler.go apps/api/internal/auth/handler_test.go apps/api/internal/auth/router.go
git commit -m "feat(auth): add HTTP handlers for login/logout/refresh/me + router"
```

---

### Task 9: Server 启动时自动迁移 + bootstrap admin

目标：把 Plan 1 的 `cmd/server/main.go` 升级：启动时运行 `migrator.Up()`；若 `users` 表为空且设置了 `BOOTSTRAP_ADMIN_EMAIL`/`BOOTSTRAP_ADMIN_PASSWORD`，创建第一个 admin；否则打印警告并继续（非阻塞）。同时注册 `/api/v1/auth/*` 路由。

**Files:**
- Modify: `apps/api/cmd/server/main.go`
- Modify: `apps/api/internal/platform/config/config.go`（加 `BootstrapAdminEmail/Password`）
- Modify: `apps/api/internal/platform/config/config_test.go`

- [ ] **Step 1: 扩展 Config 接受 bootstrap 字段**

修改 `apps/api/internal/platform/config/config.go`：在 Config struct 加 2 个字段：
```go
BootstrapAdminEmail    string
BootstrapAdminPassword string
```
在 `Load()` 从 env 读取，不强制必填：
```go
cfg.BootstrapAdminEmail = os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
cfg.BootstrapAdminPassword = os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
```

更新 config 测试：在 `TestLoad_defaults` 里多判断一句这两个字段默认为空。

- [ ] **Step 2: 跑 config 测试确保 OK**

```bash
cd apps/api && go test ./internal/platform/config/... && cd -
```

- [ ] **Step 3: 重写 cmd/server/main.go**

Replace `apps/api/cmd/server/main.go` with:
```go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	// Run pending migrations.
	mig, err := migrator.New(api.Migrations, "migrations", cfg.DatabaseURL)
	if err != nil {
		log.Error("migrator init", "error", err)
		os.Exit(1)
	}
	if err := mig.Up(); err != nil {
		log.Error("migrate up", "error", err)
		os.Exit(1)
	}
	_ = mig.Close()
	log.Info("migrations applied")

	// Open GORM handle.
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	// Build auth service.
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(auth.ServiceConfig{
		Repo:          authRepo,
		AccessSecret:  []byte(cfg.JWTAccessSecret),
		RefreshSecret: []byte(cfg.JWTRefreshSecret),
		AccessTTL:     cfg.JWTAccessTTL,
		RefreshTTL:    cfg.JWTRefreshTTL,
	})

	// Bootstrap admin if requested and users table is empty.
	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := authSvc.Bootstrap(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword, "Bootstrap Admin"); err != nil {
			cancel()
			log.Error("bootstrap admin", "error", err)
			os.Exit(1)
		}
		cancel()
		log.Info("bootstrap admin ensured", "email", cfg.BootstrapAdminEmail)
	} else {
		log.Warn("BOOTSTRAP_ADMIN_EMAIL/PASSWORD not set; skipping initial admin creation")
	}

	authHandler := auth.NewHandler(auth.HandlerConfig{
		Service:      authSvc,
		CookieSecure: cfg.CookieSecure,
		AccessTTL:    cfg.JWTAccessTTL,
		RefreshTTL:   cfg.JWTRefreshTTL,
	})

	if cfg.AppEnv != "development" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", httpx.Health(db))

	// API v1 groups.
	v1 := router.Group("/api/v1")
	public := v1.Group("")
	authed := v1.Group("")
	authed.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)), auth.CSRF())

	auth.RegisterRoutes(public, authed, authHandler)

	srv := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	idle := make(chan struct{})
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		log.Info("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown", "error", err)
		}
		close(idle)
	}()

	log.Info("api listening", "addr", cfg.HTTPListenAddr, "env", cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("listen", "error", err)
		os.Exit(1)
	}
	<-idle
	log.Info("api stopped")
}
```

- [ ] **Step 4: 编译 + 单元测试**

```bash
cd apps/api
go build ./...
go test ./...
cd -
```
Expected: 无输出；测试全绿。

- [ ] **Step 5: 本地端到端冒烟**

```bash
docker compose up -d postgres
sleep 5
cd apps/api
go build -o /tmp/tm-api ./cmd/server
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable \
JWT_ACCESS_SECRET=dev-access \
JWT_REFRESH_SECRET=dev-refresh \
BOOTSTRAP_ADMIN_EMAIL=root@example.com \
BOOTSTRAP_ADMIN_PASSWORD=change-me-on-first-login \
APP_ENV=development \
  /tmp/tm-api &
SERVER_PID=$!
sleep 3

# 登录
curl -sS -c /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"root@example.com","password":"change-me-on-first-login"}' \
     http://localhost:8080/api/v1/auth/login

# /me
curl -sS -b /tmp/tm-cookies.txt http://localhost:8080/api/v1/auth/me

kill $SERVER_PID
rm -f /tmp/tm-api
cd -
docker compose down -v
rm -f /tmp/tm-cookies.txt
```
Expected: 
- login 返回 `{"user":{"id":"...","name":"Bootstrap Admin","email":"root@example.com","role":"admin","status":"active"}}`，Set-Cookie 带 `tm_access_token` + `tm_refresh_token` + `tm_csrf_token`
- /me 返回同样的 user 对象

- [ ] **Step 6: 提交**

```bash
git add apps/api/cmd/server/main.go apps/api/internal/platform/config/
git commit -m "feat(api): auto-migrate + bootstrap admin + wire /auth/* routes"
```

---

### Task 10: 审计日志中间件 `internal/platform/audit`

**Files:**
- Create: `apps/api/internal/platform/audit/model.go`
- Create: `apps/api/internal/platform/audit/repository.go`
- Create: `apps/api/internal/platform/audit/middleware.go`
- Create: `apps/api/internal/platform/audit/middleware_test.go`

- [ ] **Step 1: model.go**

Create `apps/api/internal/platform/audit/model.go`:
```go
package audit

import (
	"time"

	"github.com/google/uuid"
)

// Log mirrors the audit_logs table.
type Log struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       *uuid.UUID     `gorm:"type:uuid" json:"user_id,omitempty"`
	Action       string         `gorm:"not null" json:"action"`
	ResourceType string         `gorm:"not null" json:"resource_type"`
	ResourceID   string         `gorm:"not null" json:"resource_id"`
	ChangesJSON  []byte         `gorm:"type:jsonb" json:"changes_json,omitempty"`
	IP           string         `gorm:"type:inet" json:"ip,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

func (Log) TableName() string { return "audit_logs" }
```

- [ ] **Step 2: repository.go**

Create `apps/api/internal/platform/audit/repository.go`:
```go
package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository persists audit log entries.
type Repository struct {
	db *gorm.DB
}

// NewRepository constructs a Repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Insert writes a single audit log entry.
func (r *Repository) Insert(ctx context.Context, l *Log) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(l).Error
}

// ListFilter holds optional filters for List.
type ListFilter struct {
	UserID       *uuid.UUID
	ResourceType string
	From         *time.Time
	To           *time.Time
	Page         int
	PageSize     int
}

// List returns audit logs matching the filter, newest first.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]Log, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 200 {
		f.PageSize = 20
	}

	q := r.db.WithContext(ctx).Model(&Log{})
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.ResourceType != "" {
		q = q.Where("resource_type = ?", f.ResourceType)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at < ?", *f.To)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var out []Log
	err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&out).Error
	return out, total, err
}
```

- [ ] **Step 3: middleware.go**

Create `apps/api/internal/platform/audit/middleware.go`:
```go
package audit

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UserProvider returns the current user's ID from a Gin context. auth package
// injects CurrentUser — this interface lets audit avoid importing auth and
// creating a dependency cycle.
type UserProvider func(c *gin.Context) (uuid.UUID, bool)

// Middleware returns a Gin middleware that writes a row to audit_logs for every
// non-safe request. Writes are best-effort: if the write fails, it logs and
// continues (never blocks the client response).
func Middleware(repo *Repository, getUserID UserProvider, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only log state-changing methods.
		if isSafe(c.Request.Method) {
			c.Next()
			return
		}

		// Capture request body for audit. Limited to 32 KiB to avoid runaway memory.
		var body []byte
		if c.Request.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(c.Request.Body, 32*1024))
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		body = scrubSensitive(body)

		c.Next()

		entry := &Log{
			Action:       c.Request.Method + " " + c.FullPath(),
			ResourceType: c.FullPath(),
			ResourceID:   c.Param("id"),
			ChangesJSON:  body,
			IP:           c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			CreatedAt:    time.Now(),
		}
		if uid, ok := getUserID(c); ok {
			entry.UserID = &uid
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := repo.Insert(ctx, entry); err != nil {
			log.Warn("audit insert failed", "error", err, "path", c.FullPath())
		}
	}
}

func isSafe(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// scrubSensitive redacts password-like fields in a JSON body. For the MVP we
// only look for the literal `"password":"..."` and replace with `"password":"[REDACTED]"`.
// This is intentionally simple; a proper parser should arrive with P6+.
func scrubSensitive(body []byte) []byte {
	return redactJSONField(body, `"password":`)
}

func redactJSONField(body []byte, needle string) []byte {
	idx := bytes.Index(body, []byte(needle))
	if idx < 0 {
		return body
	}
	start := idx + len(needle)
	// Skip whitespace and opening quote.
	for start < len(body) && (body[start] == ' ' || body[start] == '\t') {
		start++
	}
	if start >= len(body) || body[start] != '"' {
		return body
	}
	end := start + 1
	for end < len(body) && body[end] != '"' {
		if body[end] == '\\' && end+1 < len(body) {
			end += 2
			continue
		}
		end++
	}
	if end >= len(body) {
		return body
	}
	out := make([]byte, 0, len(body))
	out = append(out, body[:start+1]...)
	out = append(out, []byte("[REDACTED]")...)
	out = append(out, body[end:]...)
	return out
}
```

- [ ] **Step 4: middleware_test.go**

Create `apps/api/internal/platform/audit/middleware_test.go`:
```go
package audit_test

import (
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
)

func TestRedactPassword(t *testing.T) {
	in := []byte(`{"email":"a@b.com","password":"hunter2","remember":true}`)
	out := audit.RedactForTest(in)
	want := `{"email":"a@b.com","password":"[REDACTED]","remember":true}`
	if string(out) != want {
		t.Fatalf("got %q\nwant %q", string(out), want)
	}
}

func TestRedactWhenNoPassword(t *testing.T) {
	in := []byte(`{"email":"a@b.com"}`)
	out := audit.RedactForTest(in)
	if string(out) != string(in) {
		t.Fatalf("body must be unchanged; got %q", out)
	}
}
```

为让测试可访问内部函数，在 `apps/api/internal/platform/audit/middleware.go` 末尾加：
```go
// RedactForTest exposes scrubSensitive to tests in the same package family.
// Use only from _test.go files.
func RedactForTest(b []byte) []byte { return scrubSensitive(b) }
```

- [ ] **Step 5: 跑测试**

```bash
cd apps/api
go test ./internal/platform/audit/... -v
cd -
```
Expected: 2 条 PASS。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/platform/audit/
git commit -m "feat(audit): add audit log middleware with password redaction"
```

---

### Task 11: Admin 用户管理 endpoints

**Files:**
- Create: `apps/api/internal/auth/admin_service.go`
- Create: `apps/api/internal/auth/admin_handler.go`
- Create: `apps/api/internal/auth/admin_handler_test.go`
- Modify: `apps/api/internal/auth/router.go`（加 RegisterAdminRoutes）
- Modify: `apps/api/internal/auth/dto.go`（加 admin DTOs）

- [ ] **Step 1: 扩展 dto.go**

在 `apps/api/internal/auth/dto.go` 末尾追加：
```go
// CreateUserRequest models POST /admin/users body.
type CreateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone"`
	RoleCode string `json:"role" binding:"required,oneof=salesperson reviewer admin"`
	Password string `json:"password" binding:"required,min=8"`
}

// UpdateUserRequest models PATCH /admin/users/{id} body. All fields optional.
type UpdateUserRequest struct {
	Name     *string `json:"name"`
	Phone    *string `json:"phone"`
	RoleCode *string `json:"role" binding:"omitempty,oneof=salesperson reviewer admin"`
	Status   *string `json:"status" binding:"omitempty,oneof=active disabled"`
}

// ResetPasswordResponse is returned from the admin reset endpoint.
// Contains the freshly generated password — admin is expected to deliver it OOB.
type ResetPasswordResponse struct {
	Password string `json:"password"`
}
```

- [ ] **Step 2: admin_service.go**

Create `apps/api/internal/auth/admin_service.go`:
```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AdminService handles admin user-management use cases.
type AdminService struct {
	repo *Repository
}

// NewAdminService constructs an AdminService.
func NewAdminService(repo *Repository) *AdminService { return &AdminService{repo: repo} }

// ErrEmailTaken indicates a duplicate email on user creation.
var ErrEmailTaken = errors.New("admin: email already in use")

// CreateUser persists a new user with the given role.
func (a *AdminService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	// Duplicate email check.
	if existing, err := a.repo.FindUserByEmail(ctx, req.Email); err == nil && existing != nil {
		return nil, ErrEmailTaken
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	role, err := a.repo.FindRoleByCode(ctx, req.RoleCode)
	if err != nil {
		return nil, fmt.Errorf("find role: %w", err)
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:                uuid.New(),
		Name:              req.Name,
		Email:             req.Email,
		Phone:             req.Phone,
		PasswordHash:      hash,
		PasswordUpdatedAt: time.Now(),
		RoleID:            role.ID,
		Status:            "active",
	}
	if err := a.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	u.Role = *role
	return u, nil
}

// UpdateUser applies the non-nil fields from req to user id.
func (a *AdminService) UpdateUser(ctx context.Context, id uuid.UUID, req UpdateUserRequest) (*User, error) {
	u, err := a.repo.FindUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	changedFields := make([]string, 0, 4)
	if req.Name != nil {
		u.Name = *req.Name
		changedFields = append(changedFields, "name")
	}
	if req.Phone != nil {
		u.Phone = *req.Phone
		changedFields = append(changedFields, "phone")
	}
	if req.RoleCode != nil {
		role, err := a.repo.FindRoleByCode(ctx, *req.RoleCode)
		if err != nil {
			return nil, err
		}
		u.RoleID = role.ID
		u.Role = *role
		changedFields = append(changedFields, "role_id")
	}
	if req.Status != nil {
		u.Status = *req.Status
		changedFields = append(changedFields, "status")
	}
	if len(changedFields) == 0 {
		return u, nil
	}
	if err := a.repo.UpdateUser(ctx, u, changedFields...); err != nil {
		return nil, err
	}
	return u, nil
}

// ResetPassword generates a random password, stores its hash, and returns the plaintext
// to the admin (only this one time) so they can hand it to the user out of band.
func (a *AdminService) ResetPassword(ctx context.Context, id uuid.UUID) (string, error) {
	user, err := a.repo.FindUserByID(ctx, id)
	if err != nil {
		return "", err
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	hash, err := HashPassword(plain)
	if err != nil {
		return "", err
	}
	user.PasswordHash = hash
	user.PasswordUpdatedAt = time.Now()
	if err := a.repo.UpdateUser(ctx, user, "password_hash", "password_updated_at"); err != nil {
		return "", err
	}
	return plain, nil
}

// ListUsers delegates to the repository.
func (a *AdminService) ListUsers(ctx context.Context, q, roleCode string, page, pageSize int) ([]User, int64, error) {
	return a.repo.ListUsers(ctx, q, roleCode, page, pageSize)
}
```

- [ ] **Step 3: admin_handler.go**

Create `apps/api/internal/auth/admin_handler.go`:
```go
package auth

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHandler exposes /admin/users endpoints.
type AdminHandler struct {
	svc *AdminService
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(svc *AdminService) *AdminHandler { return &AdminHandler{svc: svc} }

// List handles GET /admin/users.
func (h *AdminHandler) List(c *gin.Context) {
	q := c.Query("q")
	role := c.Query("role")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.svc.ListUsers(c.Request.Context(), q, role, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	items := make([]UserResponse, len(users))
	for i := range users {
		items[i] = ToResponse(&users[i])
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      page,
		"page_size": size,
		"total":     total,
	})
}

// Create handles POST /admin/users.
func (h *AdminHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	u, err := h.svc.CreateUser(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			c.JSON(http.StatusConflict, gin.H{"code": "ERR_EMAIL_TAKEN", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": ToResponse(u)})
}

// Update handles PATCH /admin/users/:id.
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid id"})
		return
	}
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": err.Error()})
		return
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": ToResponse(u)})
}

// ResetPassword handles POST /admin/users/:id/reset-password.
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid id"})
		return
	}
	pw, err := h.svc.ResetPassword(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_NOT_FOUND", "message": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ResetPasswordResponse{Password: pw})
}
```

- [ ] **Step 4: 扩展 router.go**

Replace `apps/api/internal/auth/router.go` with:
```go
package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes wires the /auth/* routes.
func RegisterRoutes(public *gin.RouterGroup, authenticated *gin.RouterGroup, h *Handler) {
	public.POST("/auth/login", h.Login)
	public.POST("/auth/refresh", h.Refresh)

	authenticated.POST("/auth/logout", h.Logout)
	authenticated.GET("/auth/me", h.Me)
}

// RegisterAdminRoutes wires the /admin/users endpoints. The caller is
// responsible for applying RequireAuth and RequireRole("admin") to the group.
func RegisterAdminRoutes(g *gin.RouterGroup, h *AdminHandler) {
	g.GET("/admin/users", h.List)
	g.POST("/admin/users", h.Create)
	g.PATCH("/admin/users/:id", h.Update)
	g.POST("/admin/users/:id/reset-password", h.ResetPassword)
}
```

- [ ] **Step 5: 写 admin_handler 集成测试**

Create `apps/api/internal/auth/admin_handler_test.go`:
```go
//go:build integration

package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
)

func buildAdminRouter(t *testing.T) (*gin.Engine, *auth.Service) {
	t.Helper()
	db := freshDB(t)
	repo := auth.NewRepository(db)
	svc := auth.NewService(auth.ServiceConfig{
		Repo: repo, AccessSecret: []byte("a"), RefreshSecret: []byte("r"),
		AccessTTL: 5 * time.Minute, RefreshTTL: time.Hour,
	})
	adminSvc := auth.NewAdminService(repo)
	h := auth.NewHandler(auth.HandlerConfig{Service: svc, AccessTTL: 5 * time.Minute, RefreshTTL: time.Hour})
	ah := auth.NewAdminHandler(adminSvc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	public := r.Group("/api/v1")
	authed := r.Group("/api/v1")
	authed.Use(auth.RequireAuth([]byte("a")))
	adminOnly := r.Group("/api/v1")
	adminOnly.Use(auth.RequireAuth([]byte("a")), auth.RequireRole("admin"))

	auth.RegisterRoutes(public, authed, h)
	auth.RegisterAdminRoutes(adminOnly, ah)
	return r, svc
}

func adminCookie(t *testing.T, r *gin.Engine) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(auth.LoginRequest{Email: "admin@example.com", Password: "pw-abcdefg-1234"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d / %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieAccessToken {
			return c
		}
	}
	t.Fatalf("access cookie missing")
	return nil
}

func TestAdmin_CreateAndListUser(t *testing.T) {
	r, svc := buildAdminRouter(t)
	_ = svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin")
	cookie := adminCookie(t, r)

	body := `{"name":"Bob","email":"bob@example.com","role":"salesperson","password":"another-pw-12"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create code = %d, body = %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("list code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var listBody struct {
		Items []auth.UserResponse `json:"items"`
		Total int64               `json:"total"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &listBody)
	if listBody.Total != 2 {
		t.Fatalf("expected 2 users (admin + bob), got %d", listBody.Total)
	}
}

func TestAdmin_NonAdminForbidden(t *testing.T) {
	r, svc := buildAdminRouter(t)
	_ = svc.Bootstrap(t.Context(), "admin@example.com", "pw-abcdefg-1234", "Admin")
	cookie := adminCookie(t, r)

	// Create a salesperson user
	body := `{"name":"Sam","email":"sam@example.com","role":"salesperson","password":"another-pw-12"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Log in as sam
	samBody, _ := json.Marshal(auth.LoginRequest{Email: "sam@example.com", Password: "another-pw-12"})
	samReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(samBody))
	samReq.Header.Set("Content-Type", "application/json")
	samResp := httptest.NewRecorder()
	r.ServeHTTP(samResp, samReq)
	if samResp.Code != http.StatusOK {
		t.Fatalf("sam login failed: %d", samResp.Code)
	}
	var samCookie *http.Cookie
	for _, c := range samResp.Result().Cookies() {
		if c.Name == auth.CookieAccessToken {
			samCookie = c
		}
	}

	// Sam attempts /admin/users → 403
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	listReq.AddCookie(samCookie)
	listResp := httptest.NewRecorder()
	r.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", listResp.Code)
	}
}
```

- [ ] **Step 6: 跑所有测试**

```bash
cd apps/api
go test ./...
go test -tags=integration ./...
cd -
```
Expected: 全绿。

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/auth/admin_service.go apps/api/internal/auth/admin_handler.go apps/api/internal/auth/admin_handler_test.go apps/api/internal/auth/router.go apps/api/internal/auth/dto.go
git commit -m "feat(auth): admin user management endpoints + RBAC-gated routes"
```

---

### Task 12: Audit log admin endpoint + 接入服务器

**Files:**
- Create: `apps/api/internal/platform/audit/admin_handler.go`
- Create: `apps/api/internal/platform/audit/router.go`
- Modify: `apps/api/cmd/server/main.go`（接 audit middleware + admin 路由组）

- [ ] **Step 1: 写 admin_handler.go**

Create `apps/api/internal/platform/audit/admin_handler.go`:
```go
package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHandler exposes admin audit-log queries.
type AdminHandler struct {
	repo *Repository
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(repo *Repository) *AdminHandler { return &AdminHandler{repo: repo} }

// List handles GET /admin/audit-logs.
func (h *AdminHandler) List(c *gin.Context) {
	var f ListFilter
	f.ResourceType = c.Query("resource_type")
	f.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	f.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if uidStr := c.Query("user_id"); uidStr != "" {
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid user_id"})
			return
		}
		f.UserID = &uid
	}
	if fromStr := c.Query("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid from (expect RFC3339)"})
			return
		}
		f.From = &t
	}
	if toStr := c.Query("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_BAD_REQUEST", "message": "invalid to (expect RFC3339)"})
			return
		}
		f.To = &t
	}

	items, total, err := h.repo.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      f.Page,
		"page_size": f.PageSize,
		"total":     total,
	})
}
```

- [ ] **Step 2: 写 router.go**

Create `apps/api/internal/platform/audit/router.go`:
```go
package audit

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes registers GET /admin/audit-logs on the provided group.
// The caller is responsible for attaching RequireAuth + RequireRole("admin").
func RegisterAdminRoutes(g *gin.RouterGroup, h *AdminHandler) {
	g.GET("/admin/audit-logs", h.List)
}
```

- [ ] **Step 3: 在 main.go 里接 audit middleware + admin 路由**

修改 `apps/api/cmd/server/main.go`，在 router 搭建段补两块：

1. 在 import 里加 `"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"`
2. 在 `auth.RegisterRoutes(public, authed, authHandler)` 下面追加：

```go
	// Audit plumbing
	auditRepo := audit.NewRepository(db)
	auditMW := audit.Middleware(auditRepo, func(c *gin.Context) (uuid.UUID, bool) {
		u := auth.CurrentUser(c)
		if u.ID == uuid.Nil {
			return uuid.Nil, false
		}
		return u.ID, true
	}, log)

	// Admin routes require auth + role=admin + audit middleware + CSRF
	adminGroup := v1.Group("")
	adminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("admin"),
		auth.CSRF(),
		auditMW,
	)
	adminUserHandler := auth.NewAdminHandler(auth.NewAdminService(authRepo))
	auth.RegisterAdminRoutes(adminGroup, adminUserHandler)
	audit.RegisterAdminRoutes(adminGroup, audit.NewAdminHandler(auditRepo))
```

需要额外 import `"github.com/google/uuid"`。

- [ ] **Step 4: 编译 + 跑测试**

```bash
cd apps/api
go build ./...
go test ./...
go test -tags=integration ./...
cd -
```
Expected: 全绿。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/platform/audit/admin_handler.go apps/api/internal/platform/audit/router.go apps/api/cmd/server/main.go
git commit -m "feat(admin): expose /admin/audit-logs and wire audit middleware on admin routes"
```

---

### Task 13: 更新 docker-compose、.env.example、README

**Files:**
- Modify: `docker-compose.yml`
- Modify: `apps/api/.env.example`
- Modify: `README.md`

- [ ] **Step 1: 更新 apps/api/.env.example**

把文件改为：
```
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable
HTTP_LISTEN_ADDR=:8080
LOG_LEVEL=debug
APP_ENV=development

# Auth (required)
JWT_ACCESS_SECRET=dev-access-secret-replace-me
JWT_REFRESH_SECRET=dev-refresh-secret-replace-me
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h
COOKIE_SECURE=false

# Bootstrap admin on first startup (only used if users table is empty)
BOOTSTRAP_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_ADMIN_PASSWORD=change-me-on-first-login
```

- [ ] **Step 2: 更新 docker-compose.yml**

替换 `api.environment` 整块为：
```yaml
    environment:
      DATABASE_URL: postgres://trademark:trademark@postgres:5432/trademark?sslmode=disable
      HTTP_LISTEN_ADDR: ":8080"
      LOG_LEVEL: debug
      APP_ENV: development
      JWT_ACCESS_SECRET: dev-access-secret-replace-me
      JWT_REFRESH_SECRET: dev-refresh-secret-replace-me
      JWT_ACCESS_TTL: 15m
      JWT_REFRESH_TTL: 168h
      COOKIE_SECURE: "false"
      BOOTSTRAP_ADMIN_EMAIL: admin@example.com
      BOOTSTRAP_ADMIN_PASSWORD: change-me-on-first-login
      CORS_ORIGINS: http://localhost:5173
```

- [ ] **Step 3: 更新 README 的 "Tests" 章节末尾，追加一段 Auth smoke**

在 `## Run Locally` 的 `### Tests` 下面追加：
```markdown

### Auth smoke test (manual)

```bash
make up
sleep 15   # wait for migrations + bootstrap admin

# login; cookie jar gets tm_access_token / tm_refresh_token / tm_csrf_token
curl -sS -c /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
     http://localhost:8080/api/v1/auth/login

# current user
curl -sS -b /tmp/tm-cookies.txt http://localhost:8080/api/v1/auth/me

# admin: list users (needs CSRF for non-GET; GET omitted)
curl -sS -b /tmp/tm-cookies.txt 'http://localhost:8080/api/v1/admin/users'

make down
rm /tmp/tm-cookies.txt
```
```

- [ ] **Step 4: 提交**

```bash
git add docker-compose.yml apps/api/.env.example README.md
git commit -m "build: propagate JWT + bootstrap env vars through compose; README auth smoke"
```

---

### Task 14: 端到端冒烟（docker-compose + curl）

**Files:** （无新增，只跑验证）

- [ ] **Step 1: 冷启动**

```bash
docker compose down -v
docker compose build
docker compose up -d
sleep 25
```

- [ ] **Step 2: 验证 /health**

```bash
curl -s http://localhost:8080/health | jq .
```
Expected: `{"db":"ok","status":"ok"}`

- [ ] **Step 3: 验证登录**

```bash
curl -sS -c /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
     http://localhost:8080/api/v1/auth/login | jq .
```
Expected: `{"user":{"id":"...","name":"Bootstrap Admin","email":"admin@example.com","phone":"","role":"admin","status":"active"}}`

检查 `/tmp/tm-cookies.txt` 含 `tm_access_token` + `tm_refresh_token` + `tm_csrf_token`。

- [ ] **Step 4: 验证 /me**

```bash
curl -sS -b /tmp/tm-cookies.txt http://localhost:8080/api/v1/auth/me | jq .
```
Expected: 与 login 返回同样的 user 对象。

- [ ] **Step 5: 验证 admin list users**

```bash
curl -sS -b /tmp/tm-cookies.txt 'http://localhost:8080/api/v1/admin/users?page=1&page_size=20' | jq .
```
Expected: `{"items":[{...admin...}],"page":1,"page_size":20,"total":1}`

- [ ] **Step 6: 验证 admin create user**

从 cookies.txt 里提取 `tm_csrf_token`（admin 创建是 POST，需要 CSRF 头）：
```bash
CSRF=$(awk '$6=="tm_csrf_token"{print $7}' /tmp/tm-cookies.txt)

curl -sS -b /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -H "X-CSRF-Token: $CSRF" \
     -d '{"name":"Sam","email":"sam@example.com","role":"salesperson","password":"another-pw-12"}' \
     http://localhost:8080/api/v1/admin/users | jq .
```
Expected: 返回 201 + 新建 user JSON。

- [ ] **Step 7: 验证 non-admin 登录后访问 admin 接口被 403**

```bash
curl -sS -c /tmp/sam-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"sam@example.com","password":"another-pw-12"}' \
     http://localhost:8080/api/v1/auth/login > /dev/null

curl -sS -b /tmp/sam-cookies.txt -o /dev/null -w "%{http_code}\n" \
     http://localhost:8080/api/v1/admin/users
```
Expected: `403`

- [ ] **Step 8: 验证 audit 日志有记录**

```bash
curl -sS -b /tmp/tm-cookies.txt 'http://localhost:8080/api/v1/admin/audit-logs?page=1&page_size=20' | jq .
```
Expected: 至少 1 条记录（admin 创建 sam 的 POST 会被 audit 中间件写入）。

检查写入的 body 里 password 被 REDACTED。

- [ ] **Step 9: 清理 + 提交**

```bash
docker compose down -v
rm -f /tmp/tm-cookies.txt /tmp/sam-cookies.txt
```

Plan 2 执行期间没有新的文件改动需要提交（这是纯验证 task）。

---

## Plan 2 结束状态清单（Definition of Done）

1. ✅ `docker compose up -d` 能自动应用迁移 + 创建初始 admin。
2. ✅ POST /api/v1/auth/login 用 bootstrap admin 凭证能登录，返回用户对象 + Set-Cookie。
3. ✅ GET /api/v1/auth/me 凭 cookie 返回当前用户。
4. ✅ POST /api/v1/auth/refresh 凭 refresh cookie 返回新的 access token。
5. ✅ POST /api/v1/auth/logout 清除 cookies，后续 /me 返回 401。
6. ✅ POST /api/v1/admin/users 能由 admin 创建用户；non-admin 返回 403。
7. ✅ PATCH /api/v1/admin/users/{id} 修改用户字段；`:reset-password` 返回新密码。
8. ✅ 无 CSRF header 时非安全方法返回 403。
9. ✅ GET /api/v1/admin/audit-logs 返回审计记录，含 password-redacted body。
10. ✅ `go test ./...` 全绿；`go test -tags=integration ./...` 全绿（auth + audit + migrator + database）。
11. ✅ `make test` 全绿（前端旧测试保持绿）。

## 下一步

Plan 2 完成后进入 **Plan 3（前端 Auth 集成）**：移除 Clerk，写登录页，读 httpOnly cookie 下的用户，实现路由守卫 + TanStack Query 拦截 401 自动 refresh。
