# Monorepo + Backend Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把现有 `trademark-admin`（基于 shadcn-admin 的单体 Vite 前端）改造为 pnpm monorepo，新建 Go + Gin + GORM 后端骨架，用 docker-compose 打通本地开发环境。完成后：`docker-compose up` 成功启动 postgres + api + web 三容器；API `/health` 返回 200 并且能连上 Postgres；现有前端页面照常渲染。

**Architecture:** pnpm workspace（`apps/web` + `apps/api` + `packages/shared`）。后端用 Go modular-monolith 雏形（`internal/` 下建好空目录占位 auth / catalog / customer / pricing / quotation / export / platform）。`pkg/database` 封装 GORM + pgx 连接池。本阶段**不涉及**业务逻辑、认证、迁移脚本，只要骨架跑通。

**Tech Stack:** pnpm workspace / Go 1.23 / Gin v1.10 / GORM v1.25 / pgx v5 / PostgreSQL 16 / docker-compose v2 / 保留现有 React 19 + Vite + Clerk 栈不动（Clerk 替换放到 Plan 3）。

**上下文提示（读这个 plan 时请先明白）：**
- 当前仓库根有 `src/`、`public/`、`index.html`、`vite.config.ts`、`package.json` 等（是前端）。
- 同时存在 `package-lock.json` 和 `pnpm-lock.yaml`。项目约定用 **pnpm**，所以要删 `package-lock.json`。
- `docs/国际商标智能报价与国际业务管理平台描述.pdf` 是业务原始文档，本 plan 不需要读。
- spec 文件 `docs/superpowers/specs/2026-04-24-trademark-quote-platform-mvp-design.md` 已涵盖全景设计。
- `.claude/`、`docs/` 不能动。`.github/`、`.vscode/`、`cz.yaml`、`netlify.toml`、`CHANGELOG.md`、`LICENSE`、`README.md`、`.env.example` 留在仓库根。

---

## 全局文件结构（本 plan 结束时的仓库形态）

```
trademark-admin/
├── apps/
│   ├── web/                          ← 从仓库根迁入
│   │   ├── src/                      (原封不动)
│   │   ├── public/                   (原封不动)
│   │   ├── index.html
│   │   ├── vite.config.ts
│   │   ├── tsconfig.json
│   │   ├── tsconfig.app.json
│   │   ├── tsconfig.node.json
│   │   ├── eslint.config.js
│   │   ├── components.json
│   │   ├── knip.config.ts
│   │   ├── package.json              ← 新建，承接原根 package.json 的依赖
│   │   ├── Dockerfile                ← 新建
│   │   └── .dockerignore             ← 新建
│   └── api/                          ← 新建
│       ├── cmd/
│       │   └── server/main.go
│       ├── internal/
│       │   ├── auth/.gitkeep
│       │   ├── catalog/.gitkeep
│       │   ├── customer/.gitkeep
│       │   ├── pricing/.gitkeep
│       │   ├── quotation/.gitkeep
│       │   ├── export/.gitkeep
│       │   └── platform/
│       │       ├── config/config.go
│       │       ├── logger/logger.go
│       │       └── httpx/health.go
│       ├── pkg/
│       │   └── database/db.go
│       ├── go.mod
│       ├── go.sum
│       ├── Dockerfile
│       ├── .dockerignore
│       └── .env.example
├── packages/
│   └── shared/
│       └── .gitkeep
├── docker-compose.yml                ← 新建
├── Makefile                          ← 新建（make dev / up / down / logs）
├── pnpm-workspace.yaml               ← 新建
├── package.json                      ← 改写成 workspace root
├── .prettierrc                       (原封不动留在根)
├── .prettierignore                   (原封不动留在根)
├── pnpm-lock.yaml                    (保留)
├── .gitignore                        (追加 apps/api 相关)
├── .env.example                      (保留)
├── CHANGELOG.md / LICENSE / README.md (保留)
└── docs/                             (保留)
```

---

### Task 1: 清理 npm lockfile，确立 pnpm 为唯一包管理器

**Files:**
- Delete: `package-lock.json`
- Create: `.npmrc`

- [ ] **Step 1: 确认仓库工作区干净**

Run:
```bash
cd /Users/adam/workspace/github/trademark-admin
git status
```
Expected: `nothing to commit, working tree clean`（docs/ 下的 spec 和 plan 若已提交，也算干净）。
若有未提交更改，先和用户沟通；不要覆盖。

- [ ] **Step 2: 删除 package-lock.json**

Run:
```bash
git rm package-lock.json
```
Expected: `rm 'package-lock.json'`

- [ ] **Step 3: 创建 .npmrc 锁定 pnpm**

Create `.npmrc`:
```
engine-strict=true
auto-install-peers=true
```

- [ ] **Step 4: 提交**

```bash
git add .npmrc
git commit -m "chore: drop npm lockfile and lock project to pnpm"
```

---

### Task 2: 新建 pnpm-workspace.yaml 与 apps/packages 目录骨架

**Files:**
- Create: `pnpm-workspace.yaml`
- Create: `apps/.gitkeep`
- Create: `packages/shared/.gitkeep`

- [ ] **Step 1: 写 pnpm-workspace.yaml**

Create `pnpm-workspace.yaml`:
```yaml
packages:
  - apps/*
  - packages/*
```

- [ ] **Step 2: 建立空目录 + .gitkeep**

Run:
```bash
mkdir -p apps packages/shared
touch apps/.gitkeep packages/shared/.gitkeep
```

- [ ] **Step 3: 提交**

```bash
git add pnpm-workspace.yaml apps/.gitkeep packages/shared/.gitkeep
git commit -m "chore: scaffold pnpm monorepo layout (apps/ packages/)"
```

---

### Task 3: 迁移前端到 apps/web/（git mv 保留历史）

**Files:**
- Move all frontend files from repo root to `apps/web/`

- [ ] **Step 1: 一次性 git mv 所有前端文件**

Run（按当前 ls 得到的清单，逐一 `git mv`；不要用 shell 通配符，避免误移 docs/ 等）:
```bash
git mv src apps/web/src
git mv public apps/web/public
git mv index.html apps/web/index.html
git mv vite.config.ts apps/web/vite.config.ts
git mv tsconfig.json apps/web/tsconfig.json
git mv tsconfig.app.json apps/web/tsconfig.app.json
git mv tsconfig.node.json apps/web/tsconfig.node.json
git mv eslint.config.js apps/web/eslint.config.js
git mv components.json apps/web/components.json
git mv knip.config.ts apps/web/knip.config.ts
```

- [ ] **Step 2: 校验移动结果**

Run:
```bash
ls apps/web/
```
Expected: 包含 `src`、`public`、`index.html`、`vite.config.ts`、`tsconfig.json`、`tsconfig.app.json`、`tsconfig.node.json`、`eslint.config.js`、`components.json`、`knip.config.ts`。

```bash
ls
```
Expected 仓库根不再有上述文件；还剩 `apps/`、`packages/`、`docs/`、`.github/`、`.vscode/`、`.claude/`、`node_modules/`、`pnpm-lock.yaml`、`package.json`、`pnpm-workspace.yaml`、`.env.example`、`.gitignore`、`.prettierrc`、`.prettierignore`、`.npmrc`、`CHANGELOG.md`、`LICENSE`、`README.md`、`cz.yaml`、`netlify.toml` 等。

- [ ] **Step 3: 暂不提交**（下一任务会一并提交，避免中间状态构建不通过）

跳过 commit，进入 Task 4。

---

### Task 4: 拆分 package.json —— 前端子包 vs workspace root

**Files:**
- Create: `apps/web/package.json`
- Rewrite: `package.json`（变成纯 workspace root）

- [ ] **Step 1: 新建 apps/web/package.json**

当前仓库根的 `package.json` 是前端包，迁入 `apps/web/` 的时候要改 `name` 并去掉根工作区无关字段。

Create `apps/web/package.json`:
```json
{
  "name": "@trademark/web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "eslint .",
    "preview": "vite preview",
    "format:check": "prettier --check .",
    "format": "prettier --write .",
    "knip": "knip",
    "test": "vitest run --browser.headless",
    "test:watch": "vitest --browser.headless",
    "test:ui": "vitest --ui --browser.headless",
    "test:browser": "vitest",
    "test:coverage": "vitest run --coverage --browser.headless",
    "test:browser:install": "playwright install chromium --with-deps"
  },
  "dependencies": {
    "@clerk/react": "^6.4.2",
    "@hookform/resolvers": "^5.2.2",
    "@radix-ui/react-alert-dialog": "^1.1.15",
    "@radix-ui/react-avatar": "^1.1.11",
    "@radix-ui/react-checkbox": "^1.3.3",
    "@radix-ui/react-collapsible": "^1.1.12",
    "@radix-ui/react-dialog": "^1.1.15",
    "@radix-ui/react-direction": "^1.1.1",
    "@radix-ui/react-dropdown-menu": "^2.1.16",
    "@radix-ui/react-icons": "^1.3.2",
    "@radix-ui/react-label": "^2.1.8",
    "@radix-ui/react-popover": "^1.1.15",
    "@radix-ui/react-radio-group": "^1.3.8",
    "@radix-ui/react-scroll-area": "^1.2.10",
    "@radix-ui/react-select": "^2.2.6",
    "@radix-ui/react-separator": "^1.1.8",
    "@radix-ui/react-slot": "^1.2.4",
    "@radix-ui/react-switch": "^1.2.6",
    "@radix-ui/react-tabs": "^1.1.13",
    "@radix-ui/react-tooltip": "^1.2.8",
    "@tailwindcss/vite": "^4.2.2",
    "@tanstack/react-query": "^5.99.0",
    "@tanstack/react-router": "^1.168.22",
    "@tanstack/react-table": "^8.21.3",
    "axios": "^1.15.0",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "cmdk": "1.1.1",
    "date-fns": "^4.1.0",
    "input-otp": "^1.4.2",
    "lucide-react": "^1.8.0",
    "react": "^19.2.5",
    "react-day-picker": "9.14.0",
    "react-dom": "^19.2.5",
    "react-hook-form": "^7.72.1",
    "react-top-loading-bar": "^3.0.2",
    "recharts": "^3.8.1",
    "sonner": "^2.0.7",
    "tailwind-merge": "^3.5.0",
    "tailwindcss": "^4.2.2",
    "tw-animate-css": "^1.4.0",
    "zod": "^4.3.6",
    "zustand": "^5.0.12"
  },
  "devDependencies": {
    "@eslint/js": "^10.0.1",
    "@faker-js/faker": "^10.4.0",
    "@tanstack/eslint-plugin-query": "^5.99.0",
    "@tanstack/react-query-devtools": "^5.99.0",
    "@tanstack/react-router-devtools": "^1.166.13",
    "@tanstack/router-plugin": "^1.167.22",
    "@trivago/prettier-plugin-sort-imports": "^6.0.2",
    "@types/node": "^25.6.0",
    "@types/react": "^19.2.14",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^6.0.1",
    "@vitest/browser-playwright": "^4.1.4",
    "@vitest/coverage-v8": "^4.1.4",
    "@vitest/ui": "^4.1.4",
    "eslint": "^10.2.1",
    "eslint-plugin-react-hooks": "7.1.1",
    "eslint-plugin-react-refresh": "^0.5.2",
    "globals": "^17.5.0",
    "knip": "^6.4.1",
    "playwright": "1.59.1",
    "prettier": "^3.8.3",
    "prettier-plugin-tailwindcss": "^0.7.2",
    "typescript": "~6.0.3",
    "typescript-eslint": "^8.58.2",
    "vite": "^8.0.8",
    "vitest": "^4.1.4",
    "vitest-browser-react": "^2.2.0"
  }
}
```

- [ ] **Step 2: 重写仓库根 package.json 为 workspace root**

Rewrite root `package.json`:
```json
{
  "name": "trademark-admin",
  "private": true,
  "version": "0.0.0",
  "packageManager": "pnpm@9.0.0",
  "scripts": {
    "dev:web": "pnpm -C apps/web dev",
    "build:web": "pnpm -C apps/web build",
    "lint:web": "pnpm -C apps/web lint",
    "test:web": "pnpm -C apps/web test",
    "format:check": "prettier --check \"apps/**/*.{ts,tsx,js,jsx,json,md}\" \"packages/**/*.{ts,tsx,js,jsx,json,md}\"",
    "format": "prettier --write \"apps/**/*.{ts,tsx,js,jsx,json,md}\" \"packages/**/*.{ts,tsx,js,jsx,json,md}\""
  }
}
```

（注：`packageManager` 字段用 pnpm 9 兜底；若本地 pnpm 版本不同，发版时再调。）

- [ ] **Step 3: 清理旧 node_modules 并重新 pnpm install**

Run:
```bash
rm -rf node_modules
pnpm install
```
Expected:
- pnpm 识别到 `apps/web` 作为 workspace 包
- 依赖安装进 `apps/web/node_modules`（通过符号链接共享到 `node_modules/.pnpm`）
- 没有报错

如果 pnpm 提示「packageManager 字段版本不匹配」，用户电脑上 `pnpm -v` 是多少就把 packageManager 改成对应版本。

- [ ] **Step 4: 验证前端能 dev 启动**

Run:
```bash
pnpm -C apps/web dev
```
Expected: Vite 启动在 `http://localhost:5173`，浏览器打开能看到登录页（或现有 shadcn-admin 首页）。

启动成功后 Ctrl-C 终止。

- [ ] **Step 5: 验证前端能构建**

Run:
```bash
pnpm -C apps/web build
```
Expected: `tsc -b` 和 `vite build` 都无错。

若 TypeScript 报 `Cannot find module '...'` 之类路径问题，检查 `apps/web/tsconfig.*` 里的 `baseUrl`、`paths` 是否还指向已迁移后的相对路径（通常不需要改，因为迁的时候维持相对关系）。

- [ ] **Step 6: 提交**

```bash
git add apps/web/package.json package.json
git commit -m "chore: split package.json into apps/web workspace and root"
```

（Task 3 移动的文件已在 `git add` 中被捕获，因为 `git mv` 已 stage 它们 —— 确认：`git status` 应显示 clean。）

---

### Task 5: 写 apps/web/Dockerfile 和 .dockerignore

**Files:**
- Create: `apps/web/Dockerfile`
- Create: `apps/web/.dockerignore`

前端生产镜像：基于 node 构建 + nginx serve。

- [ ] **Step 1: 写 apps/web/Dockerfile**

Create `apps/web/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1.6

FROM node:22-alpine AS builder
WORKDIR /repo
RUN corepack enable && corepack prepare pnpm@9.0.0 --activate
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @trademark/web
COPY apps/web apps/web
RUN pnpm --filter @trademark/web build

FROM nginx:1.27-alpine AS runtime
COPY --from=builder /repo/apps/web/dist /usr/share/nginx/html
COPY apps/web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

- [ ] **Step 2: 写 apps/web/.dockerignore**

Create `apps/web/.dockerignore`:
```
node_modules
dist
.env
.env.local
coverage
.vitest-attachments
__screenshots__
```

- [ ] **Step 3: 写 apps/web/nginx.conf（SPA fallback + 代理 /api）**

Create `apps/web/nginx.conf`:
```
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location /api/ {
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

- [ ] **Step 4: 提交**

```bash
git add apps/web/Dockerfile apps/web/.dockerignore apps/web/nginx.conf
git commit -m "build(web): add Dockerfile + nginx config for production image"
```

---

### Task 6: 初始化 Go 后端模块

**Files:**
- Create: `apps/api/go.mod`
- Create: `apps/api/.gitignore`
- Create: `apps/api/.env.example`

- [ ] **Step 1: 确认 Go 版本可用**

Run:
```bash
go version
```
Expected: `go1.23` 或更高。若未安装，先 `brew install go`。

- [ ] **Step 2: 初始化 go.mod**

Run:
```bash
mkdir -p apps/api
cd apps/api
go mod init github.com/pigletfly/trademark-admin/apps/api
cd ../..
```
Expected: 生成 `apps/api/go.mod`，首行 `module github.com/pigletfly/trademark-admin/apps/api`，`go 1.23`。

- [ ] **Step 3: 为 Go 项目写 .gitignore**

Create `apps/api/.gitignore`:
```
# Binaries
bin/
server
migrate
seed

# Editor
.vscode/
.idea/

# OS
.DS_Store

# Local env
.env
.env.local
```

- [ ] **Step 4: 写 apps/api/.env.example**

Create `apps/api/.env.example`:
```
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable
HTTP_LISTEN_ADDR=:8080
LOG_LEVEL=debug
APP_ENV=development
```

- [ ] **Step 5: 提交**

```bash
git add apps/api/go.mod apps/api/.gitignore apps/api/.env.example
git commit -m "build(api): initialise Go module and env template"
```

---

### Task 7: 写配置加载模块（`internal/platform/config`）

**Files:**
- Create: `apps/api/internal/platform/config/config.go`
- Create: `apps/api/internal/platform/config/config_test.go`

模块职责：从环境变量读取配置并做必填校验。MVP 阶段先加 `DATABASE_URL`、`HTTP_LISTEN_ADDR`、`LogLevel`、`AppEnv` 四项；后续 plan 再扩展。

- [ ] **Step 1: 写失败的测试（TDD 红）**

Create `apps/api/internal/platform/config/config_test.go`:
```go
package config_test

import (
	"testing"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
)

func TestLoad_defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://a:b@localhost:5432/c?sslmode=disable")
	t.Setenv("HTTP_LISTEN_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("APP_ENV", "")

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
	if cfg.DatabaseURL == "" {
		t.Errorf("DatabaseURL must not be empty")
	}
}

func TestLoad_missingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error when DATABASE_URL is empty")
	}
}
```

- [ ] **Step 2: 验证测试失败（红）**

Run:
```bash
cd apps/api
go test ./internal/platform/config/... -v
cd ../..
```
Expected: 编译错，`package config is not found` 或 `undefined: config.Load`。

- [ ] **Step 3: 写最小实现（TDD 绿）**

Create `apps/api/internal/platform/config/config.go`:
```go
package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL    string
	HTTPListenAddr string
	LogLevel       string
	AppEnv         string
}

// Load reads configuration from environment variables and applies defaults.
// DATABASE_URL is required.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		HTTPListenAddr: getenvDefault("HTTP_LISTEN_ADDR", ":8080"),
		LogLevel:       getenvDefault("LOG_LEVEL", "info"),
		AppEnv:         getenvDefault("APP_ENV", "development"),
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: 再跑一次测试（绿）**

Run:
```bash
cd apps/api
go test ./internal/platform/config/... -v
cd ../..
```
Expected: `PASS` 两条用例都通过。

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/platform/config/
git commit -m "feat(api): add platform/config package with env loader"
```

---

### Task 8: 写 logger 模块（`internal/platform/logger`）

**Files:**
- Create: `apps/api/internal/platform/logger/logger.go`

logger 包装 stdlib `slog`，按 `LogLevel` 初始化；不涉及复杂结构，不需要专门测试文件（stdlib 行为已验证）。

- [ ] **Step 1: 写 logger.go**

Create `apps/api/internal/platform/logger/logger.go`:
```go
package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(level, env string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if strings.ToLower(env) == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
```

- [ ] **Step 2: 验证编译**

Run:
```bash
cd apps/api
go build ./internal/platform/logger/...
cd ../..
```
Expected: 无输出（success）。

- [ ] **Step 3: 提交**

```bash
git add apps/api/internal/platform/logger/
git commit -m "feat(api): add platform/logger with slog wrapper"
```

---

### Task 9: 写数据库连接模块（`pkg/database`）

**Files:**
- Create: `apps/api/pkg/database/db.go`
- Create: `apps/api/pkg/database/db_test.go`（使用 testcontainers-go）

职责：封装 GORM + pgx 建立 Postgres 连接；暴露 `Open(cfg)` 和 `Ping(db)`。

- [ ] **Step 1: 拉依赖**

Run:
```bash
cd apps/api
go get gorm.io/gorm@v1.25.12
go get gorm.io/driver/postgres@v1.5.9
go get github.com/jackc/pgx/v5@v5.7.1
cd ../..
```
Expected: go.sum 生成；go.mod 里有这三行 require。

- [ ] **Step 2: 写失败的测试**

Create `apps/api/pkg/database/db_test.go`:
```go
//go:build integration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
)

func TestOpenAndPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// postgres.Run applies a sensible default wait strategy ("database system
	// is ready to accept connections" twice) when none is supplied, so we
	// intentionally avoid overriding it here.
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("tm"),
		postgres.WithUsername("tm"),
		postgres.WithPassword("tm"),
	)
	if err != nil {
		t.Fatalf("postgres.Run: %v", err)
	}
	t.Cleanup(func() {
		_ = ctr.Terminate(ctx)
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := database.Ping(ctx, db); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
```

- [ ] **Step 3: 安装 testcontainers-go 依赖**

Run:
```bash
cd apps/api
go get github.com/testcontainers/testcontainers-go@v0.34.0
go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.34.0
cd ../..
```

- [ ] **Step 4: 验证测试失败（红）**

Run:
```bash
cd apps/api
go test -tags=integration ./pkg/database/... -v
cd ../..
```
Expected: 编译错，`undefined: database.Open` 或 `database.Ping`。

- [ ] **Step 5: 写最小实现（绿）**

Create `apps/api/pkg/database/db.go`:
```go
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open creates a GORM *gorm.DB using pgx driver.
func Open(dsn string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return gdb, nil
}

// Ping verifies the database is reachable.
func Ping(ctx context.Context, gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close releases the underlying *sql.DB. Useful on shutdown.
func Close(gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Used to ensure database/sql is a resolved import for tooling.
var _ *sql.DB
```

- [ ] **Step 6: 验证集成测试通过**

Run:
```bash
cd apps/api
go test -tags=integration ./pkg/database/... -v
cd ../..
```
Expected: 一条用例 PASS（会拉取 postgres:16-alpine 镜像，首次慢一些）。
如果本地 Docker 未运行，会报 `docker: Cannot connect to the Docker daemon` —— 先启动 Docker Desktop。

- [ ] **Step 7: 验证非集成模式编译通过**

Run:
```bash
cd apps/api
go test ./pkg/database/... -v
cd ../..
```
Expected: `ok ... [no test files]`（因为 `_test.go` 有 `//go:build integration` 标签，非 integration 模式下不编译）。

- [ ] **Step 8: 提交**

```bash
git add apps/api/pkg/database/ apps/api/go.mod apps/api/go.sum
git commit -m "feat(api): add pkg/database with gorm+pgx Open/Ping/Close"
```

---

### Task 10: 写 health 端点（`internal/platform/httpx/health.go`）

**Files:**
- Create: `apps/api/internal/platform/httpx/health.go`
- Create: `apps/api/internal/platform/httpx/health_test.go`

职责：Gin handler `GET /health`；若传入的 `db` 非 nil，额外做一次 ping；返回 `{"status":"ok","db":"ok"}` 或 503 + 原因。

- [ ] **Step 1: 拉 Gin 依赖**

Run:
```bash
cd apps/api
go get github.com/gin-gonic/gin@v1.10.0
cd ../..
```

- [ ] **Step 2: 写失败测试**

Create `apps/api/internal/platform/httpx/health_test.go`:
```go
package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
)

func TestHealth_noDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", httpx.Health(nil))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v (raw=%s)", err, w.Body.String())
	}
	if got["status"] != "ok" {
		t.Errorf("status field = %q, want ok", got["status"])
	}
	if got["db"] != "skipped" {
		t.Errorf("db field = %q, want skipped", got["db"])
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run:
```bash
cd apps/api
go test ./internal/platform/httpx/... -v
cd ../..
```
Expected: `undefined: httpx.Health`。

- [ ] **Step 4: 写实现**

Create `apps/api/internal/platform/httpx/health.go`:
```go
package httpx

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
)

// Health returns a Gin handler that reports API and (optionally) database health.
// If db is nil, db status is "skipped".
func Health(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "skipped"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(ctx, db); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "unreachable", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
	}
}
```

- [ ] **Step 5: 跑测试确认通过**

Run:
```bash
cd apps/api
go test ./internal/platform/httpx/... -v
cd ../..
```
Expected: `PASS`。

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/platform/httpx/ apps/api/go.mod apps/api/go.sum
git commit -m "feat(api): add /health endpoint with optional db ping"
```

---

### Task 11: 写 API 主入口 `cmd/server/main.go`

**Files:**
- Create: `apps/api/cmd/server/main.go`

装配：config → logger → database → gin → health 路由 → listen。优雅停机。

- [ ] **Step 1: 写 main.go**

Create `apps/api/cmd/server/main.go`:
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

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// logger not ready yet; print to stderr
		_, _ = os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	if cfg.AppEnv != "development" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", httpx.Health(db))

	srv := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
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

- [ ] **Step 2: 在内部模块里 stub 出空包**

Run:
```bash
mkdir -p apps/api/internal/auth apps/api/internal/catalog apps/api/internal/customer apps/api/internal/pricing apps/api/internal/quotation apps/api/internal/export
touch apps/api/internal/auth/.gitkeep apps/api/internal/catalog/.gitkeep apps/api/internal/customer/.gitkeep apps/api/internal/pricing/.gitkeep apps/api/internal/quotation/.gitkeep apps/api/internal/export/.gitkeep
```

这些目录后续 plan 会填充，本 plan 先占位。

- [ ] **Step 3: 编译整个后端**

Run:
```bash
cd apps/api
go build ./...
cd ../..
```
Expected: 无输出。

- [ ] **Step 4: 跑所有单元测试（非 integration）**

Run:
```bash
cd apps/api
go test ./...
cd ../..
```
Expected:
```
ok ... internal/platform/config
ok ... internal/platform/httpx
? ... internal/platform/logger [no test files]
? ... pkg/database [no test files]  (integration tagged test)
```

- [ ] **Step 5: 提交**

```bash
git add apps/api/cmd/ apps/api/internal/
git commit -m "feat(api): add server entrypoint with graceful shutdown"
```

---

### Task 12: 写 apps/api/Dockerfile + .dockerignore

**Files:**
- Create: `apps/api/Dockerfile`
- Create: `apps/api/.dockerignore`

多阶段：builder → distroless。

- [ ] **Step 1: 写 Dockerfile**

Create `apps/api/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1.6

FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=builder /out/server /app/server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
```

- [ ] **Step 2: 写 .dockerignore**

Create `apps/api/.dockerignore`:
```
bin
.env
.env.local
*.log
.git
.vscode
.idea
```

- [ ] **Step 3: 提交**

```bash
git add apps/api/Dockerfile apps/api/.dockerignore
git commit -m "build(api): add multi-stage Dockerfile (distroless runtime)"
```

---

### Task 13: 写仓库根 docker-compose.yml

**Files:**
- Create: `docker-compose.yml`

- [ ] **Step 1: 写 docker-compose.yml**

Create `docker-compose.yml`:
```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: trademark
      POSTGRES_PASSWORD: trademark
      POSTGRES_DB: trademark
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U trademark -d trademark"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build:
      context: .
      dockerfile: apps/api/Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://trademark:trademark@postgres:5432/trademark?sslmode=disable
      HTTP_LISTEN_ADDR: ":8080"
      LOG_LEVEL: debug
      APP_ENV: development
    ports:
      - "8080:8080"

  web:
    build:
      context: .
      dockerfile: apps/web/Dockerfile
    depends_on:
      - api
    ports:
      - "5173:80"

volumes:
  postgres_data:
```

- [ ] **Step 2: 本地验证（docker-compose up）**

Run:
```bash
docker compose build
docker compose up -d
```
Expected: 三个容器都 running；稍等 20 秒让 postgres 健康检查通过、api 启动。

- [ ] **Step 3: 验证 /health**

Run:
```bash
curl -s http://localhost:8080/health | jq .
```
Expected:
```json
{"db":"ok","status":"ok"}
```

若返回 `db:"unreachable"`，说明 api 比 postgres 先启动 —— 等 3 秒再试；若持续失败，看 `docker compose logs api` 。

- [ ] **Step 4: 验证前端**

在浏览器打开 `http://localhost:5173`。应能看到现有 shadcn-admin 登录页。

- [ ] **Step 5: 关停容器**

Run:
```bash
docker compose down
```

- [ ] **Step 6: 提交**

```bash
git add docker-compose.yml
git commit -m "build: add docker-compose for postgres+api+web local dev"
```

---

### Task 14: 加 Makefile 捷径

**Files:**
- Create: `Makefile`

- [ ] **Step 1: 写 Makefile**

Create `Makefile`:
```makefile
.PHONY: help install dev api web up down logs build test fmt tidy

help:
	@echo "Targets:"
	@echo "  install      pnpm install (web deps)"
	@echo "  dev          start web dev server"
	@echo "  api          run api locally (requires postgres)"
	@echo "  up           docker compose up -d (postgres+api+web)"
	@echo "  down         docker compose down"
	@echo "  logs         tail compose logs"
	@echo "  build        build all docker images"
	@echo "  test         run all tests (unit)"
	@echo "  fmt          format frontend code"
	@echo "  tidy         go mod tidy"

install:
	pnpm install

dev:
	pnpm -C apps/web dev

api:
	cd apps/api && go run ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

build:
	docker compose build

test:
	pnpm -C apps/web test
	cd apps/api && go test ./...

fmt:
	pnpm format

tidy:
	cd apps/api && go mod tidy
```

- [ ] **Step 2: 确认 make 可用**

Run:
```bash
make help
```
Expected: 打印上述目标清单。

- [ ] **Step 3: 提交**

```bash
git add Makefile
git commit -m "chore: add Makefile shortcuts (dev/up/down/test/fmt)"
```

---

### Task 15: 更新 .gitignore 收尾

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: 读取当前 .gitignore**

Run:
```bash
cat .gitignore
```

- [ ] **Step 2: 追加后端产物的忽略**

用 Edit 工具把下面几行追加到 `.gitignore` 末尾（不要重复已有行）：
```
# Go binaries
apps/api/bin/
apps/api/server
apps/api/migrate
apps/api/seed

# Test screenshots
/apps/web/__screenshots__
/apps/web/.vitest-attachments
```

- [ ] **Step 3: 提交**

```bash
git add .gitignore
git commit -m "chore: extend .gitignore for monorepo paths"
```

---

### Task 16: 最终冒烟验证 + 更新 README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 冷启动验证**

Run:
```bash
docker compose down -v
docker compose build
docker compose up -d
sleep 15
curl -s http://localhost:8080/health | jq .
```
Expected:
```json
{"db":"ok","status":"ok"}
```

再在浏览器打开 `http://localhost:5173`，确认前端登录页正常（不要登录，因 Clerk 还没切换）。

`docker compose down` 停掉。

- [ ] **Step 2: 跑完整测试**

Run:
```bash
make test
```
Expected:
- `apps/web` 的 Vitest 结果全绿（shadcn-admin 模板自带测试可能为 0，OK）
- `apps/api` 的单元测试 2 个 package PASS（config、httpx），其它 `[no test files]`。

- [ ] **Step 3: 更新 README 的 Run Locally 章节**

用 Edit 工具在 `README.md` 的 `## Run Locally` 区块替换为：
```markdown
## Run Locally

This repo is a pnpm monorepo:

- `apps/web` — React 19 frontend (Vite + TanStack Router + Shadcn)
- `apps/api` — Go 1.23 backend (Gin + GORM + PostgreSQL)
- `packages/shared` — shared types and OpenAPI schema (placeholder)

### Prerequisites

- Node 22+, pnpm 9+
- Go 1.23+
- Docker Desktop (for postgres + full stack via `docker compose`)

### One-shot dev environment

```bash
make up          # builds images, starts postgres+api+web
curl localhost:8080/health
open http://localhost:5173
make down        # stop
```

### Hot-reload dev (recommended)

```bash
docker compose up -d postgres   # only postgres
make api                        # go run backend, watches nothing — restart manually
make dev                        # vite dev server (HMR)
```

### Tests

```bash
make test
cd apps/api && go test -tags=integration ./...   # requires Docker
```
```

- [ ] **Step 4: 提交并收尾**

```bash
git add README.md
git commit -m "docs: refresh README for monorepo layout and dev setup"
```

- [ ] **Step 5: 检查最终文件布局**

Run:
```bash
ls apps/web/ apps/api/
tree apps/api/internal -L 2
```
Expected: 与本 plan 开头列的「全局文件结构」一致。

---

## Plan 1 结束状态清单（Definition of Done）

1. ✅ `docker compose up -d` 成功启动三个容器：postgres/api/web。
2. ✅ `curl http://localhost:8080/health` 返回 `{"status":"ok","db":"ok"}`。
3. ✅ 浏览器访问 `http://localhost:5173` 能看到现有 shadcn-admin 登录页。
4. ✅ `pnpm -C apps/web build` 产出 `apps/web/dist/`，无类型错误。
5. ✅ `cd apps/api && go test ./...` 单元测试全绿。
6. ✅ `cd apps/api && go test -tags=integration ./pkg/database/...` 集成测试 PASS（需 Docker）。
7. ✅ 仓库根只剩配置文件（workspace、lint、docker-compose、Makefile、docs），没有前端源码。
8. ✅ git 历史保留了 `src/` 下每个文件通过 `git mv` 迁移的连续性。

## 下一步

本 Plan 完成后，继续执行 **Plan 2（后端 Auth + Admin 用户管理）** 作为独立计划文件 `docs/superpowers/plans/2026-04-24-02-backend-auth-and-users.md`。那份 plan 的起点就是本 plan 的终点状态。
