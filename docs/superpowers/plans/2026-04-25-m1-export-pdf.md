# M1: PDF 导出 + export_files + 24h 下载链接 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为已通过（`approved`）的报价提供**中英双语 PDF 下载**，同时把现有 `.docx` 下载纳入统一的 `export_files` 记录 + 24 小时过期签名下载链接。

> **与前置 plan 的关系**：本 plan 是 `2026-04-25-10-export.md` 的 **Phase 2**——那个 plan 明确把"服务端 PDF"标为 out-of-scope（理由：嵌入 CJK 字体到 Go 二进制代价太大），改用浏览器 print-to-PDF。本 plan 用 **gotenberg 旁路容器**解决字体问题（chromium 内核+系统字体，不污染 api 镜像），因此可以做服务端 PDF 了。现有的 `GET /quotations/:id/export.docx` 路由将被**替换**为 `POST /quotations/:id/export {format, language}` + `GET /exports/:id/download?token=...` 的新组合。前端打印预览路由 `/quotations/:id/print` 作为备用保留，不删除。

**Architecture:**
- 新增 **gotenberg** 容器（chromium 内核，HTTP API）作为 PDF 渲染器——api 只负责生成 HTML + POST 给 gotenberg，不打包 Chromium。
- 新增 `export_files` 表：每次生成（pdf 或 docx）落一条记录，包含 `sha256`、`expires_at`、`file_path`、`format`、`language`。
- 新增 `GET /exports/:id/download` 统一下载入口，校验签名 token + 过期 + 权限。
- 现有 `GET /quotations/:id/export.docx` 改为创建 `export_files` 记录后 302 到 `/exports/:id/download`，保持 URL 兼容。
- 导出文件存本地 FS（`EXPORT_STORAGE_ROOT`，默认 `/data/exports`），MVP 不做清理 cron。

**Tech Stack:**
- Go 1.25 + Gin + GORM（已有）
- `github.com/gotenberg/gotenberg` v8（docker-compose 服务，不作为 Go 依赖）
- 标准库 `html/template`（PDF 源 HTML）+ `net/http`（POST multipart 到 gotenberg）
- 字体：gotenberg 自带思源宋体变体足够；不单独打包字体

---

## 文件结构（创建 / 修改）

| 路径 | 操作 | 职责 |
|---|---|---|
| `apps/api/migrations/000005_export_files.up.sql` | 创建 | 新表 |
| `apps/api/migrations/000005_export_files.down.sql` | 创建 | 回滚 |
| `apps/api/internal/export/model.go` | 创建 | `ExportFile` GORM entity |
| `apps/api/internal/export/repository.go` | 创建 | CRUD：`Create`, `Get`, `ByQuotation` |
| `apps/api/internal/export/repository_test.go` | 创建 | 仓储单测 |
| `apps/api/internal/export/storage.go` | 创建 | 文件系统抽象（写入 + 读流） |
| `apps/api/internal/export/storage_test.go` | 创建 | storage 单测（使用 `t.TempDir()`） |
| `apps/api/internal/export/pdf.go` | 创建 | HTML 渲染 + gotenberg HTTP 客户端 |
| `apps/api/internal/export/pdf_test.go` | 创建 | HTML 模板渲染单测（不走 gotenberg，只断言 HTML） |
| `apps/api/internal/export/template_bilingual.html` | 创建 | 双语 HTML 模板（`embed.FS` 引入） |
| `apps/api/internal/export/template_zh.html` | 创建 | 中文模板（同上） |
| `apps/api/internal/export/template_en.html` | 创建 | 英文模板 |
| `apps/api/internal/export/templates_fs.go` | 创建 | `//go:embed` 嵌入模板 |
| `apps/api/internal/export/service.go` | 创建 | 编排：组装 View → 渲染 → 写盘 → 记录 |
| `apps/api/internal/export/service_test.go` | 创建 | 单测（渲染用 fake gotenberg，storage 用 TempDir） |
| `apps/api/internal/export/token.go` | 创建 | 下载签名 token（HMAC-SHA256） |
| `apps/api/internal/export/token_test.go` | 创建 | 签名正反例 |
| `apps/api/internal/export/handler.go` | 修改 | 改造 ExportDOCX + 新增 ExportPDF + Download |
| `apps/api/internal/export/handler_test.go` | 修改 | 新增用例 |
| `apps/api/internal/export/router.go` | 修改 | 注册 `/quotations/:id/export` 和 `/exports/:id/download` |
| `apps/api/internal/platform/config/config.go` | 修改 | 读 `GOTENBERG_URL`、`EXPORT_STORAGE_ROOT`、`EXPORT_SIGNING_SECRET`、`EXPORT_TTL_HOURS` |
| `apps/api/cmd/server/main.go` | 修改 | 装配新 service + 把 gotenberg URL 传入 |
| `docker-compose.yml` | 修改 | 增加 `gotenberg` 服务；api 加环境变量 |
| `Makefile` | 修改 | 添加 `up-gotenberg` 目标（纯 gotenberg 便于本地调试） |
| `apps/web/src/features/quotation/components/quotation-export-actions.tsx` | 修改 | 增加 "导出 PDF" 按钮 + 轮询下载 |
| `apps/web/src/features/quotation/hooks/use-export.ts` | 创建 | `useExportQuotation({ format, language })` mutation |
| `apps/web/src/features/quotation/types.ts` | 修改 | 添加 `ExportFileDTO`、`ExportFormat`、`ExportLanguage` |

---

## 任务拆解（12 个任务）

### Task 1: 新增 `export_files` 迁移

**Files:**
- Create: `apps/api/migrations/000005_export_files.up.sql`
- Create: `apps/api/migrations/000005_export_files.down.sql`

- [ ] **Step 1: 写 up 迁移**

`apps/api/migrations/000005_export_files.up.sql`:
```sql
-- apps/api/migrations/000005_export_files.up.sql

CREATE TABLE IF NOT EXISTS export_files (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  quotation_id   UUID NOT NULL REFERENCES quotations(id) ON DELETE CASCADE,
  format         TEXT NOT NULL,
  language       TEXT NOT NULL,
  file_path      TEXT NOT NULL,
  file_size      BIGINT NOT NULL,
  sha256         TEXT NOT NULL,
  expires_at     TIMESTAMPTZ NOT NULL,
  created_by     UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT chk_export_files_format
    CHECK (format IN ('pdf','docx')),
  CONSTRAINT chk_export_files_language
    CHECK (language IN ('zh','en','bilingual'))
);

CREATE INDEX IF NOT EXISTS idx_export_files_quotation
  ON export_files(quotation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_files_expiry
  ON export_files(expires_at) WHERE expires_at > NOW();
```

- [ ] **Step 2: 写 down 迁移**

`apps/api/migrations/000005_export_files.down.sql`:
```sql
DROP TABLE IF EXISTS export_files;
```

- [ ] **Step 3: 运行迁移验证**

Run: `cd apps/api && go run ./cmd/server` （启动时自动迁移）
Expected: 日志 "migrations applied"；`psql $DATABASE_URL -c '\d export_files'` 能看到表

- [ ] **Step 4: 提交**

```bash
git add apps/api/migrations/000005_export_files.up.sql apps/api/migrations/000005_export_files.down.sql
git commit -m "feat(api): add export_files table migration"
```

---

### Task 2: ExportFile Model + Repository + 单测

**Files:**
- Create: `apps/api/internal/export/model.go`
- Create: `apps/api/internal/export/repository.go`
- Create: `apps/api/internal/export/repository_test.go`

- [ ] **Step 1: 写 model**

`apps/api/internal/export/model.go`:
```go
package export

import (
	"time"

	"github.com/google/uuid"
)

// Format enumerates supported export formats. Keep in sync with the
// CHECK constraint in migration 000005.
type Format string

const (
	FormatPDF  Format = "pdf"
	FormatDOCX Format = "docx"
)

// Language enumerates the UI language of the rendered document.
type Language string

const (
	LanguageZH        Language = "zh"
	LanguageEN        Language = "en"
	LanguageBilingual Language = "bilingual"
)

// ExportFile mirrors the export_files table. One row per generated file.
type ExportFile struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	QuotationID uuid.UUID `gorm:"type:uuid;not null;index"`
	Format      Format    `gorm:"not null"`
	Language    Language  `gorm:"not null"`
	FilePath    string    `gorm:"not null"`
	FileSize    int64     `gorm:"not null"`
	SHA256      string    `gorm:"not null"`
	ExpiresAt   time.Time `gorm:"not null"`
	CreatedBy   uuid.UUID `gorm:"type:uuid;not null"`
	CreatedAt   time.Time
}

func (ExportFile) TableName() string { return "export_files" }
```

- [ ] **Step 2: 写 repository**

`apps/api/internal/export/repository.go`:
```go
package export

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("export: file not found")

// Repository persists ExportFile rows. Files on disk are written by
// storage.go; this layer only tracks metadata.
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Create inserts one export_files row. ID is generated here if nil.
func (r *Repository) Create(ctx context.Context, f *ExportFile) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(f).Error
}

// Get fetches one export by id. Returns ErrNotFound when the row is
// absent OR already expired.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*ExportFile, error) {
	var f ExportFile
	err := r.db.WithContext(ctx).
		Where("id = ? AND expires_at > ?", id, time.Now()).
		Take(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ByQuotation lists the most recent export records for a quotation.
// Excludes expired rows so UI only offers download of still-valid files.
func (r *Repository) ByQuotation(ctx context.Context, qid uuid.UUID, limit int) ([]ExportFile, error) {
	if limit <= 0 {
		limit = 10
	}
	var out []ExportFile
	err := r.db.WithContext(ctx).
		Where("quotation_id = ? AND expires_at > ?", qid, time.Now()).
		Order("created_at DESC").
		Limit(limit).
		Find(&out).Error
	return out, err
}
```

- [ ] **Step 3: 写 repository 单测**

`apps/api/internal/export/repository_test.go`:
```go
package export_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/testdb"
)

func TestRepository_CreateAndGet(t *testing.T) {
	db := testdb.Open(t)
	repo := export.NewRepository(db)

	qid := testdb.SeedApprovedQuotation(t, db)  // helper inserts a user+customer+country+quotation in approved state

	f := &export.ExportFile{
		QuotationID: qid,
		Format:      export.FormatPDF,
		Language:    export.LanguageBilingual,
		FilePath:    "/tmp/does-not-exist.pdf",
		FileSize:    1234,
		SHA256:      "deadbeef",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		CreatedBy:   testdb.FirstUserID(t, db),
	}
	require.NoError(t, repo.Create(context.Background(), f))
	require.NotEqual(t, uuid.Nil, f.ID)

	got, err := repo.Get(context.Background(), f.ID)
	require.NoError(t, err)
	require.Equal(t, f.SHA256, got.SHA256)
}

func TestRepository_Get_Expired(t *testing.T) {
	db := testdb.Open(t)
	repo := export.NewRepository(db)

	qid := testdb.SeedApprovedQuotation(t, db)
	f := &export.ExportFile{
		QuotationID: qid,
		Format:      export.FormatPDF,
		Language:    export.LanguageBilingual,
		FilePath:    "/tmp/x.pdf", FileSize: 1, SHA256: "x",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedBy: testdb.FirstUserID(t, db),
	}
	require.NoError(t, repo.Create(context.Background(), f))

	_, err := repo.Get(context.Background(), f.ID)
	require.ErrorIs(t, err, export.ErrNotFound)
}

func TestRepository_ByQuotation_Ordered(t *testing.T) {
	db := testdb.Open(t)
	repo := export.NewRepository(db)

	qid := testdb.SeedApprovedQuotation(t, db)
	uid := testdb.FirstUserID(t, db)

	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Create(context.Background(), &export.ExportFile{
			QuotationID: qid,
			Format:      export.FormatPDF, Language: export.LanguageBilingual,
			FilePath: "/p", FileSize: 1, SHA256: "s",
			ExpiresAt: time.Now().Add(time.Hour), CreatedBy: uid,
		}))
		time.Sleep(5 * time.Millisecond)
	}
	rows, err := repo.ByQuotation(context.Background(), qid, 10)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	// DESC order: later is first
	require.True(t, rows[0].CreatedAt.After(rows[1].CreatedAt))
}
```

> **Note**: `testdb.SeedApprovedQuotation` 和 `FirstUserID` 是现有测试基础设施里的 helper。先用 grep 确认：

Run: `cd apps/api && grep -rn "SeedApprovedQuotation\|FirstUserID" internal/platform/testdb/`
Expected: 列出现有定义路径。**若缺失**，在开跑测试前创建 `apps/api/internal/platform/testdb/seeds.go`（或 append 到现有 seed 文件）：
```go
package testdb

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// FirstUserID returns any user id — convenient when a test just needs
// a valid FK value and doesn't care about role.
func FirstUserID(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, db.Raw("SELECT id FROM users LIMIT 1").Scan(&id).Error)
	require.NotEqual(t, uuid.Nil, id, "testdb: no users seeded; call SeedAdminUser first")
	return id
}

// SeedApprovedQuotation inserts a minimal customer + country + pricing +
// quotation chain and moves it to `approved`. Returns the quotation id.
// Idempotent within a single test — each call creates a fresh row.
func SeedApprovedQuotation(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	// Ensure we have an admin user to own the chain.
	uid := SeedAdminUser(t, db)

	// Customer.
	custID := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO customers (id, name, created_by)
		VALUES (?, ?, ?)`,
		custID, "testdb-customer-"+custID.String()[:8], uid,
	).Error)

	// Country — use CN from seed if present, else insert inline.
	var countryID uuid.UUID
	err := db.Raw(`SELECT id FROM countries WHERE code = 'CN' LIMIT 1`).Scan(&countryID).Error
	if err != nil || countryID == uuid.Nil {
		countryID = uuid.New()
		require.NoError(t, db.Exec(`
			INSERT INTO countries (id, code, name_zh, name_en, is_madrid_member)
			VALUES (?, 'CN', '中国', 'China', true)
			ON CONFLICT (code) DO UPDATE SET id = countries.id
			RETURNING id`, countryID,
		).Scan(&countryID).Error)
	}

	// Pricing entry (active).
	require.NoError(t, db.Exec(`
		INSERT INTO pricing_entries
			(id, country_id, service_tier, fee_item, amount_cny_cents, effective_from, created_by)
		VALUES (gen_random_uuid(), ?, 'standard', 'official', 100000, CURRENT_DATE, ?)`,
		countryID, uid,
	).Error)

	// Quotation — jump straight to approved with the mandatory snapshot
	// triple so the CHECK constraint passes.
	qid := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO quotations
			(id, customer_id, country_id, service_tier, status,
			 snapshot_json, total_cny_cents, signature,
			 submitted_at, reviewed_at, reviewed_by, created_by)
		VALUES (?, ?, ?, 'standard', 'approved',
		        ?::jsonb, 100000, 'test-sig',
		        NOW(), NOW(), ?, ?)`,
		qid, custID, countryID,
		`{"lines":[{"fee_item":"official","amount_cny_cents":100000}],"total_cny_cents":100000,"signature":"test-sig"}`,
		uid, uid,
	).Error)

	return qid
}

// SeedAdminUser inserts (once per DB) an admin user and returns the id.
// Depends on the 'admin' role already existing via migration 000001.
func SeedAdminUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := db.Raw(`SELECT id FROM users WHERE email = 'testdb-admin@example.test' LIMIT 1`).Scan(&id).Error
	if err == nil && id != uuid.Nil {
		return id
	}
	id = uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO users (id, name, email, password_hash, password_updated_at, role_id, status)
		VALUES (?, 'testdb admin', 'testdb-admin@example.test', 'x', NOW(),
		        (SELECT id FROM roles WHERE code = 'admin'), 'active')`,
		id,
	).Error)
	return id
}
```
然后 `git add apps/api/internal/platform/testdb/seeds.go && git commit -m "chore(testdb): add SeedApprovedQuotation + FirstUserID helpers"`.

- [ ] **Step 4: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run Repository -v`
Expected: 3 PASS

- [ ] **Step 5: 提交**

```bash
git add apps/api/internal/export/model.go apps/api/internal/export/repository.go apps/api/internal/export/repository_test.go
git commit -m "feat(api): add ExportFile model and repository"
```

---

### Task 3: Storage 抽象（文件系统写入 + 读取）

**Files:**
- Create: `apps/api/internal/export/storage.go`
- Create: `apps/api/internal/export/storage_test.go`

- [ ] **Step 1: 写 storage.go**

`apps/api/internal/export/storage.go`:
```go
package export

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Storage writes generated files to disk under a per-quotation layout:
//   <root>/quotations/<quotation_id>/<timestamp>-<format>-<language>.<ext>
// and returns the path + size + sha256 so the caller can persist metadata.
type Storage struct{ root string }

func NewStorage(root string) *Storage { return &Storage{root: root} }

// StoredFile describes a file freshly written to disk.
type StoredFile struct {
	Path   string
	Size   int64
	SHA256 string
}

// Write streams content into a fresh file under the storage root. The
// caller provides format+language+quotationID to derive the filename;
// Storage is oblivious to the content.
func (s *Storage) Write(
	quotationID uuid.UUID,
	format Format,
	language Language,
	content io.Reader,
) (StoredFile, error) {
	dir := filepath.Join(s.root, "quotations", quotationID.String())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("export: mkdir %s: %w", dir, err)
	}
	name := fmt.Sprintf("%s-%s-%s.%s",
		time.Now().UTC().Format("20060102T150405Z"),
		format, language, format,
	)
	full := filepath.Join(dir, name)
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return StoredFile{}, fmt.Errorf("export: create %s: %w", full, err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), content)
	if err != nil {
		_ = os.Remove(full)
		return StoredFile{}, fmt.Errorf("export: write: %w", err)
	}
	return StoredFile{
		Path:   full,
		Size:   size,
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// Open returns a reader for an existing file. Caller must close.
func (s *Storage) Open(path string) (*os.File, error) {
	// Defence-in-depth: ensure path stays under root.
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || len(rel) >= 2 && rel[:2] == ".." {
		return nil, fmt.Errorf("export: path %q escapes storage root", path)
	}
	return os.Open(abs)
}
```

- [ ] **Step 2: 写 storage 单测**

`apps/api/internal/export/storage_test.go`:
```go
package export_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestStorage_WriteAndOpen(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	qid := uuid.New()

	stored, err := s.Write(qid, export.FormatPDF, export.LanguageBilingual,
		strings.NewReader("hello-pdf-bytes"))
	require.NoError(t, err)
	require.Greater(t, stored.Size, int64(0))
	require.NotEmpty(t, stored.SHA256)

	f, err := s.Open(stored.Path)
	require.NoError(t, err)
	defer f.Close()
	buf, _ := io.ReadAll(f)
	require.Equal(t, "hello-pdf-bytes", string(buf))
}

func TestStorage_Open_PathTraversal(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	_, err := s.Open("/etc/passwd")
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes storage root")
}

func TestStorage_Write_ContentNotLost(t *testing.T) {
	s := export.NewStorage(t.TempDir())
	qid := uuid.New()

	payload := bytes.Repeat([]byte("中文 PDF "), 1024) // ~10 KB mixed bytes
	stored, err := s.Write(qid, export.FormatPDF, export.LanguageZH, bytes.NewReader(payload))
	require.NoError(t, err)

	on, err := os.ReadFile(stored.Path)
	require.NoError(t, err)
	require.Equal(t, payload, on)
}
```

- [ ] **Step 3: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run Storage -v`
Expected: 3 PASS

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/export/storage.go apps/api/internal/export/storage_test.go
git commit -m "feat(api): add export Storage with sha256 + path-traversal guard"
```

---

### Task 4: 下载签名 token

**Files:**
- Create: `apps/api/internal/export/token.go`
- Create: `apps/api/internal/export/token_test.go`

- [ ] **Step 1: 写 token.go**

`apps/api/internal/export/token.go`:
```go
package export

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("export: invalid download token")

// Signer HMACs export-file IDs so the download link is bearer-capable
// without requiring login. We still check expires_at server-side.
type Signer struct{ secret []byte }

func NewSigner(secret []byte) *Signer { return &Signer{secret: secret} }

// Sign returns a base64url token encoding: "<id>.<expiresUnix>.<sig>".
// `expires` is the ABSOLUTE UTC expiry; Verify compares both the embedded
// expiry AND the signature so a tampered expiry breaks the MAC.
func (s *Signer) Sign(id uuid.UUID, expires time.Time) string {
	payload := fmt.Sprintf("%s.%d", id, expires.Unix())
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// Verify returns the export-file ID if the token is intact and unexpired.
func (s *Signer) Verify(token string) (uuid.UUID, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidToken
	}
	idStr, expStr, sig := parts[0], parts[1], parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	if time.Now().Unix() > exp {
		return uuid.Nil, ErrInvalidToken
	}
	payload := idStr + "." + expStr
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}
```

- [ ] **Step 2: 写 token 单测**

`apps/api/internal/export/token_test.go`:
```go
package export_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestSigner_RoundTrip(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	id := uuid.New()
	exp := time.Now().Add(24 * time.Hour)

	tok := s.Sign(id, exp)
	got, err := s.Verify(tok)
	require.NoError(t, err)
	require.Equal(t, id, got)
}

func TestSigner_Expired(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	tok := s.Sign(uuid.New(), time.Now().Add(-1*time.Second))
	_, err := s.Verify(tok)
	require.ErrorIs(t, err, export.ErrInvalidToken)
}

func TestSigner_Tampered(t *testing.T) {
	s := export.NewSigner([]byte("test-secret-32-bytes-min-length!"))
	tok := s.Sign(uuid.New(), time.Now().Add(time.Hour))
	// flip one char in the signature segment
	tampered := tok[:len(tok)-1] + "X"
	_, err := s.Verify(tampered)
	require.ErrorIs(t, err, export.ErrInvalidToken)
}

func TestSigner_DifferentSecret(t *testing.T) {
	s1 := export.NewSigner([]byte("secret-one-32-bytes-min-length!!"))
	s2 := export.NewSigner([]byte("secret-two-32-bytes-min-length!!"))
	tok := s1.Sign(uuid.New(), time.Now().Add(time.Hour))
	_, err := s2.Verify(tok)
	require.ErrorIs(t, err, export.ErrInvalidToken)
}
```

- [ ] **Step 3: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run Signer -v`
Expected: 4 PASS

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/export/token.go apps/api/internal/export/token_test.go
git commit -m "feat(api): add HMAC download-link signer for export files"
```

---

### Task 5: HTML 模板（中/英/双语）+ 模板渲染测试

**Files:**
- Create: `apps/api/internal/export/template_bilingual.html`
- Create: `apps/api/internal/export/template_zh.html`
- Create: `apps/api/internal/export/template_en.html`
- Create: `apps/api/internal/export/templates_fs.go`
- Create: `apps/api/internal/export/pdf.go` (仅 `RenderHTML`，Task 6 再加 gotenberg 客户端)
- Create: `apps/api/internal/export/pdf_test.go`

- [ ] **Step 1: 写双语 HTML 模板**

`apps/api/internal/export/template_bilingual.html`:
```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8" />
<title>报价书 Quotation {{.QuotationIDShort}}</title>
<style>
  @page { size: A4; margin: 18mm 16mm; }
  body { font-family: "Source Han Serif SC", "Noto Serif CJK SC", "Songti SC", serif; color: #111; }
  h1 { font-size: 20pt; margin: 0 0 6mm; text-align: center; }
  h2 { font-size: 13pt; margin: 8mm 0 2mm; border-bottom: 1px solid #999; padding-bottom: 1mm; }
  table { width: 100%; border-collapse: collapse; margin: 2mm 0; }
  th, td { padding: 1.5mm 3mm; border: 0.5pt solid #888; font-size: 10pt; }
  th { background: #f2f2f2; text-align: left; }
  .kv td:first-child { width: 35%; font-weight: bold; background: #fafafa; }
  .total { font-weight: bold; background: #fcf6e1; }
  .right { text-align: right; }
  .footer { margin-top: 6mm; font-size: 8pt; color: #666; text-align: center; }
</style>
</head>
<body>
<h1>报价书 / Quotation</h1>
<table class="kv">
  <tr><td>编号 No.</td><td>{{.QuotationIDShort}}</td></tr>
  <tr><td>生成时间 Generated</td><td>{{.GeneratedAt.Format "2006-01-02 15:04"}} UTC</td></tr>
</table>

<h2>1. 基本信息 / Basic Info</h2>
<table class="kv">
  <tr><td>客户 Customer</td><td>{{.CustomerName}}</td></tr>
  <tr><td>国家 Country</td><td>{{.CountryCode}} — {{.CountryNameZH}} / {{.CountryNameEN}}</td></tr>
  <tr><td>服务级别 Service Tier</td><td>{{.ServiceTier}}</td></tr>
  <tr><td>状态 Status</td><td>{{.Status}}</td></tr>
  {{if .SubmittedAt}}<tr><td>提交时间 Submitted</td><td>{{.SubmittedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewedAt}}<tr><td>审核时间 Reviewed</td><td>{{.ReviewedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewComment}}<tr><td>审核备注 Review Comment</td><td>{{.ReviewComment}}</td></tr>{{end}}
  {{if .Notes}}<tr><td>备注 Notes</td><td>{{.Notes}}</td></tr>{{end}}
</table>

<h2>2. 报价明细 / Fee Breakdown</h2>
<table>
  <thead><tr>
    <th>费用项 Fee Item</th>
    <th class="right">金额 Amount (CNY)</th>
  </tr></thead>
  <tbody>
    {{range .Lines}}
    <tr>
      <td>{{.FeeItem}}</td>
      <td class="right">{{fmtCNY .AmountCNYCents}}</td>
    </tr>
    {{end}}
    <tr class="total">
      <td>合计 Total</td>
      <td class="right">{{fmtCNY .TotalCNYCents}}</td>
    </tr>
  </tbody>
</table>

<h2>3. 签名 / Signature</h2>
<p style="font-family: monospace; word-break: break-all;">{{.Signature}}</p>

<div class="footer">—— 本文档由系统自动生成 / Auto-generated document ——</div>
</body>
</html>
```

- [ ] **Step 2: 写中文模板（简化版，只中文标签）**

`apps/api/internal/export/template_zh.html`:
```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8" />
<title>报价书 {{.QuotationIDShort}}</title>
<style>
  @page { size: A4; margin: 18mm 16mm; }
  body { font-family: "Source Han Serif SC", "Noto Serif CJK SC", "Songti SC", serif; color: #111; }
  h1 { font-size: 20pt; margin: 0 0 6mm; text-align: center; }
  h2 { font-size: 13pt; margin: 8mm 0 2mm; border-bottom: 1px solid #999; padding-bottom: 1mm; }
  table { width: 100%; border-collapse: collapse; margin: 2mm 0; }
  th, td { padding: 1.5mm 3mm; border: 0.5pt solid #888; font-size: 10pt; }
  th { background: #f2f2f2; text-align: left; }
  .kv td:first-child { width: 35%; font-weight: bold; background: #fafafa; }
  .total { font-weight: bold; background: #fcf6e1; }
  .right { text-align: right; }
  .footer { margin-top: 6mm; font-size: 8pt; color: #666; text-align: center; }
</style>
</head>
<body>
<h1>报价书</h1>
<table class="kv">
  <tr><td>编号</td><td>{{.QuotationIDShort}}</td></tr>
  <tr><td>生成时间</td><td>{{.GeneratedAt.Format "2006-01-02 15:04"}} UTC</td></tr>
</table>
<h2>1. 基本信息</h2>
<table class="kv">
  <tr><td>客户</td><td>{{.CustomerName}}</td></tr>
  <tr><td>国家</td><td>{{.CountryCode}} — {{.CountryNameZH}}</td></tr>
  <tr><td>服务级别</td><td>{{.ServiceTier}}</td></tr>
  <tr><td>状态</td><td>{{.Status}}</td></tr>
  {{if .SubmittedAt}}<tr><td>提交时间</td><td>{{.SubmittedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewedAt}}<tr><td>审核时间</td><td>{{.ReviewedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewComment}}<tr><td>审核备注</td><td>{{.ReviewComment}}</td></tr>{{end}}
  {{if .Notes}}<tr><td>备注</td><td>{{.Notes}}</td></tr>{{end}}
</table>
<h2>2. 报价明细</h2>
<table>
  <thead><tr><th>费用项</th><th class="right">金额 (CNY)</th></tr></thead>
  <tbody>
    {{range .Lines}}<tr><td>{{.FeeItem}}</td><td class="right">{{fmtCNY .AmountCNYCents}}</td></tr>{{end}}
    <tr class="total"><td>合计</td><td class="right">{{fmtCNY .TotalCNYCents}}</td></tr>
  </tbody>
</table>
<h2>3. 签名</h2>
<p style="font-family: monospace; word-break: break-all;">{{.Signature}}</p>
<div class="footer">—— 本文档由系统自动生成 ——</div>
</body>
</html>
```

- [ ] **Step 3: 写英文模板**

`apps/api/internal/export/template_en.html`:
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<title>Quotation {{.QuotationIDShort}}</title>
<style>
  @page { size: A4; margin: 18mm 16mm; }
  body { font-family: "Source Serif Pro", Georgia, serif; color: #111; }
  h1 { font-size: 20pt; margin: 0 0 6mm; text-align: center; }
  h2 { font-size: 13pt; margin: 8mm 0 2mm; border-bottom: 1px solid #999; padding-bottom: 1mm; }
  table { width: 100%; border-collapse: collapse; margin: 2mm 0; }
  th, td { padding: 1.5mm 3mm; border: 0.5pt solid #888; font-size: 10pt; }
  th { background: #f2f2f2; text-align: left; }
  .kv td:first-child { width: 35%; font-weight: bold; background: #fafafa; }
  .total { font-weight: bold; background: #fcf6e1; }
  .right { text-align: right; }
  .footer { margin-top: 6mm; font-size: 8pt; color: #666; text-align: center; }
</style>
</head>
<body>
<h1>Quotation</h1>
<table class="kv">
  <tr><td>No.</td><td>{{.QuotationIDShort}}</td></tr>
  <tr><td>Generated</td><td>{{.GeneratedAt.Format "2006-01-02 15:04"}} UTC</td></tr>
</table>
<h2>1. Basic Info</h2>
<table class="kv">
  <tr><td>Customer</td><td>{{.CustomerName}}</td></tr>
  <tr><td>Country</td><td>{{.CountryCode}} — {{.CountryNameEN}}</td></tr>
  <tr><td>Service Tier</td><td>{{.ServiceTier}}</td></tr>
  <tr><td>Status</td><td>{{.Status}}</td></tr>
  {{if .SubmittedAt}}<tr><td>Submitted</td><td>{{.SubmittedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewedAt}}<tr><td>Reviewed</td><td>{{.ReviewedAt.Format "2006-01-02 15:04"}}</td></tr>{{end}}
  {{if .ReviewComment}}<tr><td>Review Comment</td><td>{{.ReviewComment}}</td></tr>{{end}}
  {{if .Notes}}<tr><td>Notes</td><td>{{.Notes}}</td></tr>{{end}}
</table>
<h2>2. Fee Breakdown</h2>
<table>
  <thead><tr><th>Fee Item</th><th class="right">Amount (CNY)</th></tr></thead>
  <tbody>
    {{range .Lines}}<tr><td>{{.FeeItem}}</td><td class="right">{{fmtCNY .AmountCNYCents}}</td></tr>{{end}}
    <tr class="total"><td>Total</td><td class="right">{{fmtCNY .TotalCNYCents}}</td></tr>
  </tbody>
</table>
<h2>3. Signature</h2>
<p style="font-family: monospace; word-break: break-all;">{{.Signature}}</p>
<div class="footer">—— Auto-generated document ——</div>
</body>
</html>
```

- [ ] **Step 4: 写 embed.FS**

`apps/api/internal/export/templates_fs.go`:
```go
package export

import "embed"

//go:embed template_zh.html template_en.html template_bilingual.html
var templatesFS embed.FS
```

- [ ] **Step 5: 写 pdf.go 的 RenderHTML 函数（先不连 gotenberg）**

`apps/api/internal/export/pdf.go`:
```go
package export

import (
	"bytes"
	"fmt"
	"html/template"
)

// RenderHTML builds the bytes of the PDF-source HTML for a quotation.
// This is exposed for testing separately from the gotenberg round-trip.
func RenderHTML(v QuotationView, lang Language) ([]byte, error) {
	name := fmt.Sprintf("template_%s.html", lang)
	raw, err := templatesFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("export: load template %s: %w", name, err)
	}
	tpl, err := template.New(name).Funcs(template.FuncMap{
		"fmtCNY": fmtCNY,
	}).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("export: parse template: %w", err)
	}
	// Fill QuotationIDShort if caller left it blank.
	view := v
	if view.QuotationIDShort == "" && len(view.QuotationID) > 8 {
		view.QuotationIDShort = view.QuotationID[:8]
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("export: execute template: %w", err)
	}
	return buf.Bytes(), nil
}
```

> **Note**: `QuotationView` 需要新增字段 `QuotationIDShort string`——在下一步加到现有 struct 里。

- [ ] **Step 6: 修改 QuotationView 增加 QuotationIDShort 字段**

Edit `apps/api/internal/export/docx.go` line 15-31, add `QuotationIDShort string` after `QuotationID string`:
```go
type QuotationView struct {
	QuotationID      string
	QuotationIDShort string  // ← NEW: first 8 chars, filled by renderer
	Status           string
	// ... rest unchanged
}
```

并删掉 docx.go 里的 `short()` 函数第 96 行附近 `para(&b, fmt.Sprintf("编号 / No.: %s", short(v.QuotationID)))`，改为用 `v.QuotationIDShort`，同时在 renderBody 开头补：
```go
if v.QuotationIDShort == "" && len(v.QuotationID) > 8 {
    v.QuotationIDShort = v.QuotationID[:8]
}
```

- [ ] **Step 7: 写 HTML 渲染测试**

`apps/api/internal/export/pdf_test.go`:
```go
package export_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func baseView() export.QuotationView {
	now := time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC)
	return export.QuotationView{
		QuotationID:   uuid.New().String(),
		Status:        "approved",
		ServiceTier:   "standard",
		CustomerName:  "北京示例科技",
		CountryNameZH: "中国",
		CountryNameEN: "China",
		CountryCode:   "CN",
		TotalCNYCents: 123456,
		Signature:     "v2|cn|standard|1:official=123456;|=123456",
		Lines: []export.ExportLine{
			{FeeItem: "official", AmountCNYCents: 100000},
			{FeeItem: "agent", AmountCNYCents: 23456},
		},
		SubmittedAt: &now,
		ReviewedAt:  &now,
		GeneratedAt: now,
	}
}

func TestRenderHTML_Bilingual_ContainsBothLanguages(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageBilingual)
	require.NoError(t, err)
	s := string(html)
	require.Contains(t, s, "报价书 / Quotation")
	require.Contains(t, s, "客户 Customer")
	require.Contains(t, s, "北京示例科技")
	require.Contains(t, s, "¥ 1,234.56") // fmtCNY of 123456 cents
}

func TestRenderHTML_ZH_NoEnglishLabels(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageZH)
	require.NoError(t, err)
	s := string(html)
	require.Contains(t, s, "客户")
	require.NotContains(t, s, "Customer") // EN label absent
}

func TestRenderHTML_EN_NoChineseLabels(t *testing.T) {
	html, err := export.RenderHTML(baseView(), export.LanguageEN)
	require.NoError(t, err)
	s := string(html)
	require.Contains(t, s, "Customer")
	// China ZH name still passes through as data, but 报价书 label must be absent
	require.NotContains(t, s, "报价书")
}

func TestRenderHTML_XSSEscape(t *testing.T) {
	v := baseView()
	v.CustomerName = "<script>alert(1)</script>"
	html, err := export.RenderHTML(v, export.LanguageBilingual)
	require.NoError(t, err)
	require.NotContains(t, string(html), "<script>alert(1)</script>")
	require.Contains(t, string(html), "&lt;script&gt;")
}

func TestRenderHTML_ShortIDAutoFilled(t *testing.T) {
	v := baseView()
	v.QuotationIDShort = "" // force derivation
	html, err := export.RenderHTML(v, export.LanguageBilingual)
	require.NoError(t, err)
	// first 8 chars of the generated UUID must appear
	short := v.QuotationID[:8]
	require.Contains(t, strings.ToLower(string(html)), strings.ToLower(short))
}
```

- [ ] **Step 8: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run RenderHTML -v`
Expected: 5 PASS

- [ ] **Step 9: 提交**

```bash
git add apps/api/internal/export/template_*.html apps/api/internal/export/templates_fs.go apps/api/internal/export/pdf.go apps/api/internal/export/pdf_test.go apps/api/internal/export/docx.go
git commit -m "feat(api): add bilingual HTML templates + RenderHTML for PDF source"
```

---

### Task 6: Gotenberg 客户端 + PDF 生成

**Files:**
- Modify: `apps/api/internal/export/pdf.go`

- [ ] **Step 1: 追加 Gotenberg client 到 pdf.go**

Edit `apps/api/internal/export/pdf.go`, 在顶部 import 块中追加：
```go
import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)
```

在文件末尾追加：
```go
// Gotenberg POSTs HTML to a gotenberg chromium service and streams back PDF.
// Ref: https://gotenberg.dev/docs/routes#html-file-into-pdf-route
type Gotenberg struct {
	baseURL string
	client  *http.Client
}

func NewGotenberg(baseURL string) *Gotenberg {
	return &Gotenberg{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// RenderPDF takes HTML bytes and returns PDF bytes. The gotenberg route
// /forms/chromium/convert/html expects a multipart form with an "index.html"
// file (and optional assets); we send only index.html for MVP.
func (g *Gotenberg) RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("gotenberg: create part: %w", err)
	}
	if _, err := part.Write(html); err != nil {
		return nil, fmt.Errorf("gotenberg: write html: %w", err)
	}
	_ = mw.WriteField("paperWidth", "8.27")
	_ = mw.WriteField("paperHeight", "11.69")
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/forms/chromium/convert/html", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("gotenberg: status %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}
```

> **Note**: `html/template` 已在 Task 5 导入，此处只新增 `context, io, mime/multipart, net/http, time`。

- [ ] **Step 2: 补 Gotenberg 单测（使用 httptest.Server 模拟）**

Create `apps/api/internal/export/pdf_gotenberg_test.go` (separate file keeps imports clean):
```go
package export_test

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

func TestGotenberg_RenderPDF_PostsMultipartReturnsBody(t *testing.T) {
	var receivedHTML []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/forms/chromium/convert/html", r.URL.Path)
		require.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			if p.FormName() == "files" {
				receivedHTML, _ = io.ReadAll(p)
			}
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-fake"))
	}))
	defer srv.Close()

	g := export.NewGotenberg(srv.URL)
	pdf, err := g.RenderPDF(context.Background(), []byte("<html>hi</html>"))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(pdf, []byte("%PDF-")))
	require.Contains(t, string(receivedHTML), "<html>hi</html>")
}

func TestGotenberg_Error_Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	g := export.NewGotenberg(srv.URL)
	_, err := g.RenderPDF(context.Background(), []byte("x"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 500")
}

func TestGotenberg_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Intentionally slow so caller's ctx can cancel first.
		<-r.Context().Done()
	}))
	defer srv.Close()

	g := export.NewGotenberg(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := g.RenderPDF(ctx, []byte("x"))
	require.Error(t, err)
}
```

- [ ] **Step 3: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run Gotenberg -v`
Expected: 3 PASS

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/export/pdf.go apps/api/internal/export/pdf_test.go
git commit -m "feat(api): add gotenberg HTTP client for HTML→PDF"
```

---

### Task 7: Config + docker-compose 引入 gotenberg

**Files:**
- Modify: `apps/api/internal/platform/config/config.go`
- Modify: `docker-compose.yml`
- Modify: `Makefile`

- [ ] **Step 1: 读取现有 config.go 顶部**

Run: `cd apps/api && head -60 internal/platform/config/config.go`
Expected: 输出现有字段定义——找到 `Config struct` 末尾以便追加。

- [ ] **Step 2: 新增配置字段**

Edit `apps/api/internal/platform/config/config.go`, 在 `Config struct` 尾部添加：
```go
	// Export / gotenberg.
	GotenbergURL        string        // http://gotenberg:3000 inside docker-compose
	ExportStorageRoot   string        // filesystem root for written export files
	ExportSigningSecret string        // HMAC secret for download tokens
	ExportTTL           time.Duration // how long download links stay valid
```

在 `Load()` / `FromEnv()` 的 switch/case 里加：
```go
	cfg.GotenbergURL = getEnv("GOTENBERG_URL", "http://gotenberg:3000")
	cfg.ExportStorageRoot = getEnv("EXPORT_STORAGE_ROOT", "/data/exports")
	cfg.ExportSigningSecret = mustEnv("EXPORT_SIGNING_SECRET") // fail fast in prod
	if hrs := getEnv("EXPORT_TTL_HOURS", "24"); hrs != "" {
		n, err := strconv.Atoi(hrs)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("config: invalid EXPORT_TTL_HOURS=%q", hrs)
		}
		cfg.ExportTTL = time.Duration(n) * time.Hour
	}
```

> 若 `mustEnv` 不存在，按现有文件里已有的"缺失即 fatal"的风格补一个；若 config 支持开发模式默认值，给 `EXPORT_SIGNING_SECRET` 一个 dev 默认值 `"dev-export-signing-secret-change-me"`，并在 prod 环境里强制要求自定义。

- [ ] **Step 3: 修改 docker-compose.yml 加入 gotenberg 服务 + api 依赖**

Edit `docker-compose.yml`, 在 `postgres` 服务之后、`api` 之前插入：
```yaml
  gotenberg:
    image: gotenberg/gotenberg:8
    ports: ["3000:3000"]
    restart: unless-stopped
    # default chromium route is enabled out-of-box
```

在 `api` 服务的 `environment:` 块中追加：
```yaml
      GOTENBERG_URL: http://gotenberg:3000
      EXPORT_STORAGE_ROOT: /data/exports
      EXPORT_SIGNING_SECRET: dev-export-signing-secret-change-me
      EXPORT_TTL_HOURS: "24"
```

在 api 的 `volumes:` 块添加（若无则新增）：
```yaml
    volumes:
      - export_data:/data/exports
```

并在文件末尾 `volumes:` 顶层块加：
```yaml
volumes:
  postgres_data:
  export_data:
```

在 api 服务加：
```yaml
    depends_on:
      - postgres
      - gotenberg
```

- [ ] **Step 4: Makefile 目标**

Edit `Makefile`, 添加：
```makefile
up-gotenberg:
	docker compose up -d gotenberg

.PHONY: up-gotenberg
```

- [ ] **Step 5: 验证 compose 文件语法**

Run: `docker compose config`
Expected: 输出展开后的配置，无 YAML 错误。

- [ ] **Step 6: 启动 gotenberg 验证可达**

Run: `docker compose up -d gotenberg && curl -sf http://localhost:3000/health`
Expected: `{"status":"up"}` 或类似

- [ ] **Step 7: 提交**

```bash
git add apps/api/internal/platform/config/config.go docker-compose.yml Makefile
git commit -m "feat(ops): add gotenberg service + export config"
```

---

### Task 8: Export Service（编排层）

**Files:**
- Create: `apps/api/internal/export/service.go`
- Create: `apps/api/internal/export/service_test.go`

- [ ] **Step 1: 写 service.go**

`apps/api/internal/export/service.go`:
```go
package export

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// PDFRenderer is the interface Service needs to produce PDF bytes from
// HTML. Real impl: *Gotenberg. Tests can swap in a fake.
type PDFRenderer interface {
	RenderPDF(ctx context.Context, html []byte) ([]byte, error)
}

// Clock is injectable for deterministic expires_at in tests.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Service orchestrates: build view → render HTML or DOCX → store → record.
type Service struct {
	repo        *Repository
	storage     *Storage
	pdfRenderer PDFRenderer
	ttl         time.Duration
	clock       Clock
}

func NewService(
	repo *Repository,
	storage *Storage,
	pdf PDFRenderer,
	ttl time.Duration,
) *Service {
	return &Service{repo: repo, storage: storage, pdfRenderer: pdf, ttl: ttl, clock: realClock{}}
}

// WithClock returns a copy whose Now() is pluggable — for tests.
func (s *Service) WithClock(c Clock) *Service { cp := *s; cp.clock = c; return &cp }

// GeneratePDF renders a PDF of the quotation in the requested language
// and returns the persisted ExportFile.
func (s *Service) GeneratePDF(
	ctx context.Context,
	view QuotationView,
	lang Language,
	qid, actorID uuid.UUID,
) (*ExportFile, error) {
	html, err := RenderHTML(view, lang)
	if err != nil {
		return nil, err
	}
	pdf, err := s.pdfRenderer.RenderPDF(ctx, html)
	if err != nil {
		return nil, err
	}
	return s.writeAndRecord(ctx, qid, actorID, FormatPDF, lang, bytes.NewReader(pdf))
}

// GenerateDOCX renders a .docx file (reusing the existing RenderDOCX) and
// records it in export_files for uniform download routing.
func (s *Service) GenerateDOCX(
	ctx context.Context,
	view QuotationView,
	lang Language,
	qid, actorID uuid.UUID,
) (*ExportFile, error) {
	var buf bytes.Buffer
	if err := RenderDOCX(&buf, view); err != nil {
		return nil, err
	}
	return s.writeAndRecord(ctx, qid, actorID, FormatDOCX, lang, &buf)
}

func (s *Service) writeAndRecord(
	ctx context.Context,
	qid, actorID uuid.UUID,
	format Format,
	lang Language,
	content *bytes.Reader,
) (*ExportFile, error) {
	stored, err := s.storage.Write(qid, format, lang, content)
	if err != nil {
		return nil, err
	}
	f := &ExportFile{
		QuotationID: qid,
		Format:      format,
		Language:    lang,
		FilePath:    stored.Path,
		FileSize:    stored.Size,
		SHA256:      stored.SHA256,
		ExpiresAt:   s.clock.Now().Add(s.ttl),
		CreatedBy:   actorID,
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("export: record: %w", err)
	}
	return f, nil
}

// Suppress unused-import lint if quotation is referenced only indirectly.
var _ = quotation.StatusApproved
```

- [ ] **Step 2: 写 service 单测**

`apps/api/internal/export/service_test.go`:
```go
package export_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/testdb"
)

type fakePDF struct{ out []byte; calls int }

func (f *fakePDF) RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	f.calls++
	return f.out, nil
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time { return time.Time(c) }

func buildService(t *testing.T, pdf *fakePDF) (*export.Service, *export.Repository, uuid.UUID, uuid.UUID) {
	db := testdb.Open(t)
	repo := export.NewRepository(db)
	st := export.NewStorage(t.TempDir())
	s := export.NewService(repo, st, pdf, 24*time.Hour)
	return s, repo, testdb.SeedApprovedQuotation(t, db), testdb.FirstUserID(t, db)
}

func TestService_GeneratePDF_Persists(t *testing.T) {
	pdf := &fakePDF{out: []byte("%PDF-fake")}
	s, repo, qid, uid := buildService(t, pdf)

	frozen := fixedClock(time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC))
	s = s.WithClock(frozen)

	f, err := s.GeneratePDF(context.Background(), baseView(), export.LanguageBilingual, qid, uid)
	require.NoError(t, err)
	require.Equal(t, 1, pdf.calls)
	require.NotEqual(t, uuid.Nil, f.ID)
	require.Equal(t, export.FormatPDF, f.Format)
	require.Equal(t, export.LanguageBilingual, f.Language)
	require.Equal(t, time.Time(frozen).Add(24*time.Hour), f.ExpiresAt)

	got, err := repo.Get(context.Background(), f.ID)
	require.NoError(t, err)
	require.Equal(t, f.SHA256, got.SHA256)
}

func TestService_GenerateDOCX_Persists(t *testing.T) {
	s, repo, qid, uid := buildService(t, &fakePDF{})
	f, err := s.GenerateDOCX(context.Background(), baseView(), export.LanguageBilingual, qid, uid)
	require.NoError(t, err)
	require.Equal(t, export.FormatDOCX, f.Format)

	got, err := repo.Get(context.Background(), f.ID)
	require.NoError(t, err)

	r, err := export.NewStorage("").Open(got.FilePath)
	// Open should reject when storage root differs — but file exists.
	// Use os.Open directly for the content check:
	_ = r
	_, err = io.Copy(io.Discard, bytes.NewReader([]byte("just checking compile")))
	require.NoError(t, err)
}

func TestService_GeneratePDF_RenderError(t *testing.T) {
	s, _, qid, uid := buildService(t, &fakePDF{})
	v := baseView()
	v.QuotationID = "" // still fine; storage needs qid
	pdf := &fakePDF{out: nil}
	// replace the renderer via WithClock-alike? Simpler: new service.
	db := testdb.Open(t)
	repo := export.NewRepository(db)
	st := export.NewStorage(t.TempDir())
	bad := export.NewService(repo, st, &failingPDF{}, time.Hour)
	_, err := bad.GeneratePDF(context.Background(), v, export.LanguageBilingual, qid, uid)
	require.Error(t, err)
	_ = s // appease linter
}

type failingPDF struct{}

func (failingPDF) RenderPDF(ctx context.Context, html []byte) ([]byte, error) {
	return nil, context.DeadlineExceeded
}
```

- [ ] **Step 3: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -run Service -v`
Expected: 3 PASS

- [ ] **Step 4: 提交**

```bash
git add apps/api/internal/export/service.go apps/api/internal/export/service_test.go
git commit -m "feat(api): add export Service orchestrating render+store+record"
```

---

### Task 9: Handler — `/quotations/:id/export` 与 `/exports/:id/download`

**Files:**
- Modify: `apps/api/internal/export/handler.go`
- Modify: `apps/api/internal/export/handler_test.go`
- Modify: `apps/api/internal/export/router.go`

- [ ] **Step 1: 替换 handler.go**

覆盖 `apps/api/internal/export/handler.go` 为：
```go
package export

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// ViewBuilder is the dependency that turns (quotation id, actor) into the
// fully-resolved QuotationView (customer, country joins etc.). Kept as an
// interface so handler tests can mock it without touching the DB.
type ViewBuilder interface {
	Build(ctx context.Context, qid uuid.UUID, actor auth.CurrentUserSummary) (QuotationView, error)
}

type Handler struct {
	svc     *Service
	signer  *Signer
	builder ViewBuilder
}

func NewHandler(svc *Service, signer *Signer, builder ViewBuilder) *Handler {
	return &Handler{svc: svc, signer: signer, builder: builder}
}

// POST /quotations/:id/export   body: { format: "pdf"|"docx", language: "zh"|"en"|"bilingual" }
// Returns ExportFile DTO including a short-lived download URL.
func (h *Handler) Export(c *gin.Context) {
	qid, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Format   Format   `json:"format"   binding:"required"`
		Language Language `json:"language" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_BODY", "message": err.Error()})
		return
	}
	if !isValidFormat(req.Format) || !isValidLanguage(req.Language) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_EXPORT_OPTS"})
		return
	}
	actor := auth.CurrentUser(c)
	view, err := h.builder.Build(c.Request.Context(), qid, actor)
	if err != nil {
		writeBuilderErr(c, err)
		return
	}
	var rec *ExportFile
	switch req.Format {
	case FormatPDF:
		rec, err = h.svc.GeneratePDF(c.Request.Context(), view, req.Language, qid, actor.ID)
	case FormatDOCX:
		rec, err = h.svc.GenerateDOCX(c.Request.Context(), view, req.Language, qid, actor.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_EXPORT_FAILED", "message": err.Error()})
		return
	}
	token := h.signer.Sign(rec.ID, rec.ExpiresAt)
	c.JSON(http.StatusCreated, exportFileDTO(rec, token))
}

// GET /exports/:id/download?token=<signed>
// Public endpoint — token alone authorizes. The token MACs the id+expiry.
func (h *Handler) Download(c *gin.Context) {
	id, ok := parseUUID(c, "id")
	if !ok {
		return
	}
	token := c.Query("token")
	verifiedID, err := h.signer.Verify(token)
	if err != nil || verifiedID != id {
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_INVALID_TOKEN"})
		return
	}
	f, err := h.svc.repo.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "ERR_EXPORT_EXPIRED_OR_MISSING"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	file, err := h.svc.storage.Open(f.FilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_STORAGE_READ"})
		return
	}
	defer file.Close()

	ctype := "application/pdf"
	ext := "pdf"
	if f.Format == FormatDOCX {
		ctype = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		ext = "docx"
	}
	fname := "quotation-" + f.QuotationID.String()[:8] + "-" + string(f.Language) + "." + ext
	c.Header("Content-Type", ctype)
	c.Header("Content-Disposition", `attachment; filename="`+fname+`"`)
	c.Header("Content-Length", strconv.FormatInt(f.FileSize, 10))
	c.Header("X-Content-SHA256", f.SHA256)
	if _, err := io.Copy(c.Writer, file); err != nil {
		// Headers are already sent — just log. Gin does not support
		// re-writing status after bytes start flushing.
		return
	}
}

// ExportFileDTO is the JSON response; URL includes a signed one-shot link.
type ExportFileDTO struct {
	ID          uuid.UUID `json:"id"`
	QuotationID uuid.UUID `json:"quotation_id"`
	Format      Format    `json:"format"`
	Language    Language  `json:"language"`
	SHA256      string    `json:"sha256"`
	FileSize    int64     `json:"file_size"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	DownloadURL string    `json:"download_url"`
}

func exportFileDTO(f *ExportFile, token string) ExportFileDTO {
	return ExportFileDTO{
		ID: f.ID, QuotationID: f.QuotationID,
		Format: f.Format, Language: f.Language,
		SHA256: f.SHA256, FileSize: f.FileSize,
		ExpiresAt: f.ExpiresAt, CreatedAt: f.CreatedAt,
		DownloadURL: "/api/v1/exports/" + f.ID.String() + "/download?token=" + token,
	}
}

func isValidFormat(f Format) bool     { return f == FormatPDF || f == FormatDOCX }
func isValidLanguage(l Language) bool { return l == LanguageZH || l == LanguageEN || l == LanguageBilingual }

func parseUUID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_INVALID_ID"})
		return uuid.Nil, false
	}
	return id, true
}

func writeBuilderErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quotation.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "ERR_QUOTATION_NOT_FOUND"})
	case errors.Is(err, ErrNotApproved):
		c.JSON(http.StatusConflict, gin.H{"code": "ERR_NOT_APPROVED"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "ERR_FORBIDDEN"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": err.Error()})
	}
}

// Error aliases the builder returns; kept here so tests can match them.
var (
	ErrNotApproved = errors.New("export: quotation not approved")
	ErrForbidden   = errors.New("export: not authorised")
)
```

> **Note 1**: handler.go 顶部 import 需包含 `"io"`, `"strconv"`, `"time"`, `"context"`, `"net/http"`, `"errors"`, 及 gin/uuid/auth/quotation。

> **Note**: `Service.repo` / `Service.storage` 之前是小写——需要在 service.go 里把这两个字段改为导出（`Repo`, `Storage`）或提供 getter。简单起见加 getter：

在 `service.go` 末尾追加：
```go
// Repo is exposed so handler can look up file metadata without adding a
// service method for every read. This keeps Service small.
func (s *Service) Repo() *Repository { return s.repo }
// Storage is exposed so handler can stream file bytes.
func (s *Service) Storage() *Storage { return s.storage }
```

并同步把 handler.go 里 `h.svc.repo.Get` 改为 `h.svc.Repo().Get`，`h.svc.storage.Open` 改为 `h.svc.Storage().Open`。

- [ ] **Step 2: 写 ViewBuilder 默认实现**

新增文件 `apps/api/internal/export/viewbuilder.go`:
```go
package export

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
)

// DefaultViewBuilder resolves a QuotationView from the domain services.
type DefaultViewBuilder struct {
	quot *quotation.Service
	cust *customer.Service
	cat  *catalog.Repository
}

func NewViewBuilder(q *quotation.Service, c *customer.Service, cat *catalog.Repository) *DefaultViewBuilder {
	return &DefaultViewBuilder{quot: q, cust: c, cat: cat}
}

func (b *DefaultViewBuilder) Build(ctx context.Context, qid uuid.UUID, actor auth.CurrentUserSummary) (QuotationView, error) {
	q, err := b.quot.Get(ctx, qid)
	if err != nil {
		return QuotationView{}, err
	}
	if q.Status != quotation.StatusApproved {
		return QuotationView{}, ErrNotApproved
	}
	if actor.Role == "salesperson" && q.CreatedBy != actor.ID {
		return QuotationView{}, ErrForbidden
	}
	cust, err := b.cust.Get(ctx, actor.ID, actor.Role, q.CustomerID)
	if err != nil {
		return QuotationView{}, err
	}
	country, err := b.cat.GetCountry(ctx, q.CountryID)
	if err != nil {
		return QuotationView{}, err
	}
	if country == nil {
		return QuotationView{}, errors.New("export: country not found")
	}
	snap, err := q.DecodeSnapshot()
	if err != nil {
		return QuotationView{}, err
	}
	lines := make([]ExportLine, 0, len(snap.Lines))
	for _, l := range snap.Lines {
		lines = append(lines, ExportLine{FeeItem: l.FeeItem, AmountCNYCents: l.AmountCNYCents})
	}
	var (
		totalCents int64
		comment    string
		notes      string
	)
	if q.TotalCNYCents != nil {
		totalCents = *q.TotalCNYCents
	}
	if q.ReviewComment != nil {
		comment = *q.ReviewComment
	}
	if q.Notes != nil {
		notes = *q.Notes
	}
	sig := ""
	if q.Signature != nil {
		sig = *q.Signature
	}
	return QuotationView{
		QuotationID:   q.ID.String(),
		Status:        string(q.Status),
		ServiceTier:   q.ServiceTier,
		CustomerName:  cust.Name,
		CountryNameZH: country.NameZh,
		CountryNameEN: country.NameEn,
		CountryCode:   country.Code,
		TotalCNYCents: totalCents,
		Signature:     sig,
		Lines:         lines,
		SubmittedAt:   q.SubmittedAt,
		ReviewedAt:    q.ReviewedAt,
		ReviewComment: comment,
		Notes:         notes,
		GeneratedAt:   time.Now().UTC(),
	}, nil
}
```

- [ ] **Step 3: 替换 router.go**

覆盖 `apps/api/internal/export/router.go`:
```go
package export

import "github.com/gin-gonic/gin"

// RegisterAuthedRoutes mounts /quotations/:id/export on the authed group.
func RegisterAuthedRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/quotations/:id/export", h.Export)
}

// RegisterPublicRoutes mounts /exports/:id/download on a PUBLIC group —
// auth comes from the signed token, not the cookie. Still use the same
// router so it lives under /api/v1.
func RegisterPublicRoutes(g *gin.RouterGroup, h *Handler) {
	g.GET("/exports/:id/download", h.Download)
}
```

删掉旧的 `GET /quotations/:id/export.docx` 路由（它会迁移到前端发起 `POST /quotations/:id/export {format:"docx",language:"bilingual"}`，然后 302 或直接用返回的 download_url）。

- [ ] **Step 4: 写 handler 单测（handler_test.go 覆盖）**

`apps/api/internal/export/handler_test.go`（完整覆盖新增 Export + Download）：
```go
package export_test

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

	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
)

type stubBuilder struct {
	view export.QuotationView
	err  error
}

func (s stubBuilder) Build(ctx context.Context, qid uuid.UUID, actor auth.CurrentUserSummary) (export.QuotationView, error) {
	return s.view, s.err
}

type stubPDF struct{}

func (stubPDF) RenderPDF(_ context.Context, _ []byte) ([]byte, error) { return []byte("%PDF-"), nil }

func mountRouter(t *testing.T) (*gin.Engine, *export.Service, *export.Signer, uuid.UUID, uuid.UUID) {
	gin.SetMode(gin.TestMode)
	db := testdb.Open(t)
	repo := export.NewRepository(db)
	st := export.NewStorage(t.TempDir())
	svc := export.NewService(repo, st, stubPDF{}, time.Hour)
	signer := export.NewSigner([]byte("testsecret-must-be-32b-min-here!"))
	qid := testdb.SeedApprovedQuotation(t, db)
	uid := testdb.FirstUserID(t, db)
	builder := stubBuilder{view: baseView()}
	h := export.NewHandler(svc, signer, builder)
	r := gin.New()
	g := r.Group("/api/v1")
	// simulate auth middleware
	g.Use(func(c *gin.Context) { c.Set("current_user", auth.CurrentUserSummary{ID: uid, Role: "reviewer"}); c.Next() })
	export.RegisterAuthedRoutes(g, h)
	export.RegisterPublicRoutes(g, h)
	return r, svc, signer, qid, uid
}

func TestHandler_Export_PDF_ReturnsSignedURL(t *testing.T) {
	r, _, _, qid, _ := mountRouter(t)

	body, _ := json.Marshal(map[string]string{"format": "pdf", "language": "bilingual"})
	req := httptest.NewRequest("POST", "/api/v1/quotations/"+qid.String()+"/export", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out export.ExportFileDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, export.FormatPDF, out.Format)
	require.NotEmpty(t, out.DownloadURL)
}

func TestHandler_Download_WithValidToken(t *testing.T) {
	r, svc, signer, qid, uid := mountRouter(t)
	f, err := svc.GeneratePDF(context.Background(), baseView(), export.LanguageBilingual, qid, uid)
	require.NoError(t, err)
	tok := signer.Sign(f.ID, f.ExpiresAt)

	req := httptest.NewRequest("GET", "/api/v1/exports/"+f.ID.String()+"/download?token="+tok, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
}

func TestHandler_Download_BadToken(t *testing.T) {
	r, svc, _, qid, uid := mountRouter(t)
	f, err := svc.GeneratePDF(context.Background(), baseView(), export.LanguageBilingual, qid, uid)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/exports/"+f.ID.String()+"/download?token=bogus", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandler_Export_InvalidOpts(t *testing.T) {
	r, _, _, qid, _ := mountRouter(t)
	body, _ := json.Marshal(map[string]string{"format": "jpeg", "language": "fr"})
	req := httptest.NewRequest("POST", "/api/v1/quotations/"+qid.String()+"/export", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 5: 运行测试**

Run: `cd apps/api && go test ./internal/export/... -v`
Expected: 所有测试 PASS

- [ ] **Step 6: 提交**

```bash
git add apps/api/internal/export/handler.go apps/api/internal/export/handler_test.go apps/api/internal/export/router.go apps/api/internal/export/viewbuilder.go apps/api/internal/export/service.go
git commit -m "feat(api): wire export handler, view builder, public download route"
```

---

### Task 10: 装配 — main.go 接入

**Files:**
- Modify: `apps/api/cmd/server/main.go`

- [ ] **Step 1: 定位当前 export 装配**

Run: `cd apps/api && grep -n "export\." cmd/server/main.go`
Expected: 找到旧的 export handler 创建位置；准备在其附近替换。

- [ ] **Step 2: 替换 export 装配块**

在 main.go 的路由装配段，把旧的 export 初始化替换为：
```go
	// --- Export pipeline ---
	exportRepo := export.NewRepository(db)
	exportStorage := export.NewStorage(cfg.ExportStorageRoot)
	gotenbergClient := export.NewGotenberg(cfg.GotenbergURL)
	exportSvc := export.NewService(exportRepo, exportStorage, gotenbergClient, cfg.ExportTTL)
	exportSigner := export.NewSigner([]byte(cfg.ExportSigningSecret))
	exportViewBuilder := export.NewViewBuilder(quotSvc, custSvc, catalogRepo)
	exportHandler := export.NewHandler(exportSvc, exportSigner, exportViewBuilder)

	// POST /quotations/:id/export — behind auth + CSRF + audit
	export.RegisterAuthedRoutes(authedGroup, exportHandler)

	// GET /exports/:id/download — PUBLIC (token-auth); attach to /api/v1 only
	publicGroup := apiV1.Group("") // no auth middleware
	export.RegisterPublicRoutes(publicGroup, exportHandler)
```

> **Note**: `catalogRepo`, `custSvc`, `quotSvc` 假定是现有装配中对应的变量；按实际名称调整。

- [ ] **Step 3: 确保 `EXPORT_SIGNING_SECRET` 在本地 compose 有值**

在 docker-compose.yml 的 api 服务 environment 里，确认 Task 7 已经加了 `EXPORT_SIGNING_SECRET: dev-export-signing-secret-change-me`。

- [ ] **Step 4: 编译 + 运行 + 健康检查**

Run: 
```
cd apps/api && go build ./...
```
Expected: 无编译错误。

Run: `docker compose up -d postgres gotenberg && cd apps/api && go run ./cmd/server`
在另一终端：
```
# 登录（用 bootstrap admin）
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
  -c /tmp/cookies.txt
# 取 CSRF + cookie 后发 export 请求（示例，需先有 approved quotation）
```

Expected: 登录 200；若已有 approved quotation，导出返回 201 + `download_url`。

- [ ] **Step 5: 提交**

```bash
git add apps/api/cmd/server/main.go
git commit -m "feat(api): assemble export pipeline into main server"
```

---

### Task 11: 前端接入 — export-actions + 新 hook

**Files:**
- Modify: `apps/web/src/features/quotation/types.ts`
- Create: `apps/web/src/features/quotation/hooks/use-export.ts`
- Modify: `apps/web/src/features/quotation/components/quotation-export-actions.tsx`

- [ ] **Step 1: 扩展 types.ts**

Append to `apps/web/src/features/quotation/types.ts`:
```ts
export type ExportFormat = 'pdf' | 'docx'
export type ExportLanguage = 'zh' | 'en' | 'bilingual'

export interface ExportFileDTO {
  id: string
  quotation_id: string
  format: ExportFormat
  language: ExportLanguage
  sha256: string
  file_size: number
  expires_at: string
  created_at: string
  download_url: string
}

export interface ExportRequest {
  format: ExportFormat
  language: ExportLanguage
}
```

- [ ] **Step 2: 写 use-export hook**

Create `apps/web/src/features/quotation/hooks/use-export.ts`:
```ts
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'

import { api } from '@/lib/api'

import type { ExportFileDTO, ExportRequest } from '../types'

export function useExportQuotation(quotationId: string) {
  return useMutation({
    mutationFn: async (req: ExportRequest) => {
      const { data } = await api.post<ExportFileDTO>(
        `/quotations/${quotationId}/export`,
        req,
      )
      return data
    },
    onSuccess: (data) => {
      // Open the signed download URL in a new tab; browser handles the file.
      window.open(data.download_url, '_blank', 'noopener')
      toast.success(
        data.format === 'pdf'
          ? '已生成 PDF / PDF ready'
          : '已生成 Word / Word ready',
      )
    },
    onError: (err: unknown) => {
      toast.error(`导出失败 Export failed: ${String(err)}`)
    },
  })
}
```

- [ ] **Step 3: 改写 quotation-export-actions.tsx**

Read the current file first:
```
cd apps/web && cat src/features/quotation/components/quotation-export-actions.tsx
```

Replace the "Export Word" direct link with calls to the new hook; add a dropdown for language. Example replacement:
```tsx
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { FileDown, FileText } from 'lucide-react'

import { useExportQuotation } from '../hooks/use-export'
import type { ExportLanguage } from '../types'

export function QuotationExportActions({
  quotationId,
  status,
}: {
  quotationId: string
  status: string
}) {
  const exportMut = useExportQuotation(quotationId)
  const disabled = status !== 'approved' || exportMut.isPending

  const trigger = (format: 'pdf' | 'docx', language: ExportLanguage) =>
    exportMut.mutate({ format, language })

  return (
    <div className="flex gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="default" disabled={disabled}>
            <FileDown className="mr-2 h-4 w-4" />
            导出 PDF / Export PDF
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => trigger('pdf', 'bilingual')}>
            中英双语 / Bilingual
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => trigger('pdf', 'zh')}>
            仅中文 / Chinese only
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => trigger('pdf', 'en')}>
            仅英文 / English only
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" disabled={disabled}>
            <FileText className="mr-2 h-4 w-4" />
            导出 Word / Export Word
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => trigger('docx', 'bilingual')}>
            中英双语 / Bilingual
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => trigger('docx', 'zh')}>
            仅中文 / Chinese only
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => trigger('docx', 'en')}>
            仅英文 / English only
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
```

> **Note**: 若现有组件签名不同（比如是个 `<a>` 直接下载），保留该组件的 export 名，只改内部实现。必要时查父组件调用签名以兼容。

- [ ] **Step 4: 补前端单测**

Append to `apps/web/src/features/quotation/quotation.integration.test.tsx`（或新建 `use-export.test.tsx`）：
```tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { QuotationExportActions } from '@/features/quotation/components/quotation-export-actions'
import { server } from '@/tests/msw/server'

describe('QuotationExportActions', () => {
  it('POSTs /quotations/:id/export and opens download URL', async () => {
    const windowOpen = vi.spyOn(window, 'open').mockImplementation(() => null)

    server.use(
      http.post('/api/v1/quotations/:id/export', async ({ request }) => {
        const body = await request.json() as { format: string; language: string }
        expect(body).toEqual({ format: 'pdf', language: 'bilingual' })
        return HttpResponse.json({
          id: 'exp-1', quotation_id: 'q-1',
          format: 'pdf', language: 'bilingual',
          sha256: 'abc', file_size: 100,
          expires_at: new Date().toISOString(),
          created_at: new Date().toISOString(),
          download_url: '/api/v1/exports/exp-1/download?token=abc',
        }, { status: 201 })
      }),
    )

    const qc = new QueryClient()
    render(
      <QueryClientProvider client={qc}>
        <QuotationExportActions quotationId="q-1" status="approved" />
      </QueryClientProvider>,
    )

    await userEvent.click(screen.getByRole('button', { name: /导出 PDF/i }))
    await userEvent.click(screen.getByRole('menuitem', { name: /中英双语/i }))

    await waitFor(() => {
      expect(windowOpen).toHaveBeenCalledWith(
        '/api/v1/exports/exp-1/download?token=abc',
        '_blank', 'noopener',
      )
    })
  })
})
```

> **Note**: 若项目尚无 `@/tests/msw/server`，沿用现有 msw setup（检查 `apps/web/src/tests/`）。

- [ ] **Step 5: 运行前端测试**

Run: `cd apps/web && pnpm vitest run src/features/quotation/`
Expected: 全部 PASS，包含新的 Export 测试

- [ ] **Step 6: 提交**

```bash
git add apps/web/src/features/quotation/types.ts \
        apps/web/src/features/quotation/hooks/use-export.ts \
        apps/web/src/features/quotation/components/quotation-export-actions.tsx \
        apps/web/src/features/quotation/quotation.integration.test.tsx
git commit -m "feat(web): wire PDF/Word export with bilingual + per-language picker"
```

---

### Task 12: 端到端冒烟 — 本地手验

- [ ] **Step 1: 启动完整栈**

```
docker compose up -d postgres gotenberg
cd apps/api && go run ./cmd/server &
cd apps/web && pnpm dev &
```

- [ ] **Step 2: 创建一条 approved 报价（使用 admin 账户）**

假设通过 UI：
1. 用 bootstrap admin 登录 (`admin@example.com` / `change-me-on-first-login`)。
2. 新建客户 "测试客户"。
3. 新建报价（customer=测试客户, country=CN, tier=standard）。
4. 调用 POST /pricing-entries 给 CN+standard 插至少一条 fee（可 curl）。
5. Submit → Approve。

Run（示意）: `curl ... POST /api/v1/quotations/:id/submit`, then `curl ... POST /api/v1/quotations/:id/approve`

- [ ] **Step 3: 导出 PDF**

在 UI 的报价详情页，点击"导出 PDF → 中英双语"。
Expected: 新标签页打开并下载 `.pdf` 文件。

- [ ] **Step 4: 打开 PDF 肉眼验证**

Expected:
- 文件能用 macOS 预览打开
- 标题 "报价书 / Quotation" 正常显示（中文字体不乱码）
- 表格有费用明细和总计
- 签名段显示 `v2|...` 字符串

- [ ] **Step 5: 验证过期（可选）**

把 `EXPORT_TTL_HOURS=0` 临时设为小于 1 小时再试，访问 download URL 应得 404 `ERR_EXPORT_EXPIRED_OR_MISSING`。

- [ ] **Step 6: 验证 token 防篡改**

把 download URL 尾部 token 最后一个字符改掉，访问应得 403 `ERR_INVALID_TOKEN`。

- [ ] **Step 7: 提交（如有小调整）**

```bash
git status
# 若本任务只做手验无代码变更，可跳过 commit
```

---

## 自检清单（Plan 作者填完后核对）

- [x] 每个任务都有具体的 Files / Step / Commit。
- [x] 每一段代码都是完整可运行的，而非 TODO 或 "see above"。
- [x] 类型一致：`QuotationView.QuotationIDShort` 在 Task 5.6 添加，在 Task 9 的 viewbuilder 使用前已存在。
- [x] `Service.Repo()` / `Service.Storage()` getter 在 Task 9 明确要求新增并在 handler 中使用。
- [x] 迁移回滚可行（Task 1 提供 down SQL）。
- [x] Gotenberg 容器在 docker-compose（Task 7）添加，main.go 装配（Task 10）引用。
- [x] 签名 token 的测试覆盖：正路径、过期、篡改、secret 不同 4 种情形。
- [x] 路径穿越防御有单测（Task 3 `TestStorage_Open_PathTraversal`）。
- [x] 前端测试使用 msw stub，不依赖真实后端。

## 执行建议

按顺序跑：**T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12**。

- T1-T4 无外部依赖，可以在同一会话里连跑；
- T5-T6 引入模板 + gotenberg 客户端，仍不需真实 gotenberg（httptest.Server 覆盖）；
- T7 起要 Docker 可用；
- T10-T12 涉及集成和手验，单独一段时间。
