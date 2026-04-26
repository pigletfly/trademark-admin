# M5: Playwright E2E 全链路 — 实装计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在新 workspace 包 `packages/e2e/` 下建立 4 个 Playwright spec，覆盖登录 → 客户 → 向导 → 提交 → 审核 → 调价 → 通过 → 下载 PDF 的完整业务链路，本地 `make e2e` 一键跑通。

**Architecture:** `packages/e2e/` 作为独立 pnpm workspace 包，使用 `@playwright/test` ^1.59（匹配 web 已锁的 `playwright@1.59.1`）。4 个 spec 按前缀数字排序串行执行：`01-admin-setup` → `02-salesperson-journey` → `03-reviewer-journey` → `04-export`。跨 spec 通信走 `state/test-run.json`（gitignored），用随机 8-hex 后缀保证多次运行的数据不冲突。`fixtures/api-client.ts` 封装真实 admin API 调用（登录 + 建用户/pricing/customer），处理 `tm_csrf_token` cookie ↔ `X-CSRF-Token` header 的双提交。`fixtures/pages/*` 是 4 个 Page Object（Login/List/Wizard/Detail）。

**Tech Stack:** TypeScript + `@playwright/test` ^1.59 · Chromium only · workers=1 · serial 执行 · 依赖已启动的 `docker compose up -d`（api :8080 / web :5173 / postgres :5432 / gotenberg :3000）。

**前置**：M1（导出）+ M2（审核流）+ M3（向导）+ M4（溯源）均已完成并落盘。

---

## File Structure

```
packages/e2e/
├── package.json              # 新建 — @trademark/e2e, private, 声明 @playwright/test
├── tsconfig.json             # 新建 — 合理 strict 配置（不继承，独立一份）
├── playwright.config.ts      # 新建 — baseURL=localhost:5173, workers=1, chromium-only, trace/video retain-on-failure
├── .gitignore                # 新建 — state/ test-results/ playwright-report/ node_modules/
├── README.md                 # 新建 — 怎么跑、怎么调试、state 语义说明
├── fixtures/
│   ├── api-client.ts         # 新建 — loginAdmin()/createUser()/createPricingEntry()/createCustomer(); 封 CSRF+cookie
│   ├── test-data.ts          # 新建 — randomHex(8), testUsers(suffix), testCustomer(suffix), stateFile helpers
│   └── pages/
│       ├── login.page.ts     # 新建 — LoginPage(page): goto() / signIn(email, password)
│       ├── list.page.ts      # 新建 — ListPage(page): goto() / clickNew() / openById(id)
│       ├── wizard.page.ts    # 新建 — WizardPage(page): selectCustomer() / selectCountry() / selectTier() / fillNotes() / next() / back() / saveAndSubmit()
│       └── detail.page.ts    # 新建 — DetailPage(page): expectStatus() / openAdjustSheet() / saveAdjust() / approve() / exportPdfBilingual() / waitForDownload()
├── tests/
│   ├── 01-admin-setup.spec.ts       # 新建
│   ├── 02-salesperson-journey.spec.ts  # 新建
│   ├── 03-reviewer-journey.spec.ts     # 新建
│   └── 04-export.spec.ts            # 新建
└── state/                     # gitignored 目录 — spec 写入 test-run.json
    └── .gitkeep               # 新建 — 占位让目录入库
```

**同时修改：**
- `Makefile`（根）— 新增 `e2e` 目标，预检 `http://localhost:8080/health` 后调 `pnpm -C packages/e2e test`

**不需要修改：**
- `pnpm-workspace.yaml` 已含 `packages/*`，自动识别新包
- 后端代码、前端代码零改动

---

## Task 1: 脚手架 + 预检

**Files:**
- Create: `packages/e2e/package.json`
- Create: `packages/e2e/tsconfig.json`
- Create: `packages/e2e/playwright.config.ts`
- Create: `packages/e2e/.gitignore`
- Create: `packages/e2e/state/.gitkeep`
- Create: `packages/e2e/README.md`
- Modify: `Makefile` (add `e2e` target)

- [ ] **Step 1: 创建 `packages/e2e/package.json`**

```json
{
  "name": "@trademark/e2e",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "test": "playwright test",
    "test:headed": "playwright test --headed",
    "test:ui": "playwright test --ui",
    "report": "playwright show-report",
    "install:browsers": "playwright install chromium --with-deps"
  },
  "devDependencies": {
    "@playwright/test": "1.59.1",
    "typescript": "^5.6.0"
  }
}
```

锁版本 1.59.1：与 `apps/web` 的 `playwright@1.59.1` 保持一致，避免 browser binary 重复下载。

- [ ] **Step 2: 创建 `packages/e2e/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "resolveJsonModule": true,
    "types": ["@playwright/test", "node"],
    "lib": ["ES2022", "DOM"]
  },
  "include": ["fixtures/**/*", "tests/**/*", "playwright.config.ts"]
}
```

独立一份、不继承根配置：E2E 代码跑在 node runtime，跟 web 的 DOM/Vite config 不一样。

- [ ] **Step 3: 创建 `packages/e2e/playwright.config.ts`**

```ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // 前后依赖靠 state/test-run.json，必须串行
  workers: 1,
  retries: 0, // 本地跑失败直接看报告；CI 不接入
  timeout: 30_000, // 单测 30s 足够
  expect: { timeout: 5_000 },
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 10_000,
    navigationTimeout: 10_000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
  ],
})
```

- [ ] **Step 4: 创建 `packages/e2e/.gitignore`**

```
node_modules/
test-results/
playwright-report/
state/*.json
!state/.gitkeep
```

- [ ] **Step 5: 创建 `packages/e2e/state/.gitkeep`**

空文件。让 `state/` 目录在 clone 后存在，`test-run.json` 本身被忽略。

- [ ] **Step 6: 创建 `packages/e2e/README.md`**

```markdown
# @trademark/e2e

Playwright E2E covering the full business happy path:
login → customer → wizard → submit → review → adjust → approve → download PDF.

## Prerequisites

Start the full stack first:

    docker compose up -d

Then verify the API is healthy:

    curl http://localhost:8080/health

The compose file already provides a bootstrap admin
(`admin@example.com` / `change-me-on-first-login`) — no extra seeding needed.

Before the first run install the browser binary:

    pnpm -C packages/e2e install:browsers

## Running

From repo root:

    make e2e                # preflight + run all 4 specs
    pnpm -C packages/e2e test           # skip preflight
    pnpm -C packages/e2e test:headed    # watch the browser
    pnpm -C packages/e2e test:ui        # interactive UI mode
    pnpm -C packages/e2e report         # open last HTML report

## Execution order

Tests run serially via `fullyParallel: false` + `workers: 1`. Files are picked
up in filename order:

1. `01-admin-setup.spec.ts` — bootstrap admin logs in via API, creates 2
   users (salesperson, reviewer), 2 pricing entries, 1 customer. Writes
   `state/test-run.json`.
2. `02-salesperson-journey.spec.ts` — UI login as salesperson, runs the 5-step
   wizard, saves + submits. Writes `quotationId` back into the state file.
3. `03-reviewer-journey.spec.ts` — UI login as reviewer, adjusts pricing,
   approves.
4. `04-export.spec.ts` — UI login as salesperson, triggers PDF bilingual
   export, asserts the download URL request fires.

If any spec fails, later specs won't find the expected state and will fail
with "Run 01-admin-setup first" or similar. Rerun from `01-` after fixing.

## Data isolation

Each run generates a random 8-hex suffix (e.g. `ab12cd34`). All created
entities carry it (`salesperson-ab12cd34@example.com`, `客户-ab12cd34 有限公司`, etc).
Multiple runs accumulate data in the DB but never collide. If you want a
clean DB:

    docker compose down -v && docker compose up -d

## Debugging a failure

    pnpm -C packages/e2e report     # HTML: screenshots, video, trace viewer

Click through to the failing test for trace + screenshot + console output.
```

- [ ] **Step 7: 修改根 `Makefile` 加 `e2e` 目标**

在 `.PHONY` 行追加 `e2e`，在 `up-gotenberg:` 目标下方新增：

```makefile
e2e:
	@echo "→ Checking api health at http://localhost:8080/health"
	@curl -fsS http://localhost:8080/health > /dev/null 2>&1 || ( \
		echo "API not responding. Run 'docker compose up -d' first."; exit 1 \
	)
	@echo "→ Checking web at http://localhost:5173"
	@curl -fsS http://localhost:5173/ > /dev/null 2>&1 || ( \
		echo "Web not responding. Run 'docker compose up -d' first."; exit 1 \
	)
	pnpm -C packages/e2e test
```

更新 help 文本：在 `up-gotenberg` 行之后加 `@echo "  e2e           run Playwright E2E (requires docker compose up -d)"`。

- [ ] **Step 8: 安装依赖 + 下载浏览器**

```bash
pnpm install
pnpm -C packages/e2e install:browsers
```

预期：`packages/e2e/node_modules/` 生成、`@playwright/test` 解析成功；chromium 二进制下载。

- [ ] **Step 9: 验证 playwright 能识别配置**

```bash
pnpm -C packages/e2e exec playwright test --list
```

预期输出：`No tests found in tests/` —— tests 目录还没文件，但 config 能正确被识别说明脚手架 OK。

- [ ] **Step 10: Commit**

```bash
git add packages/e2e Makefile
git commit -m "$(cat <<'EOF'
feat(e2e): scaffold packages/e2e with Playwright config + Makefile target

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: API Client + Test Data Fixtures

**Files:**
- Create: `packages/e2e/fixtures/test-data.ts`
- Create: `packages/e2e/fixtures/api-client.ts`

- [ ] **Step 1: 创建 `packages/e2e/fixtures/test-data.ts`**

```ts
import { randomBytes } from 'node:crypto'
import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import path from 'node:path'

const STATE_PATH = path.resolve(__dirname, '..', 'state', 'test-run.json')

// 8-hex suffix (~4 billion values). Good enough to avoid collisions even if
// you rerun the E2E 100+ times without `docker compose down -v`.
export function randomHex(bytes = 4): string {
  return randomBytes(bytes).toString('hex')
}

export interface TestState {
  suffix: string
  adminEmail: string
  adminPassword: string
  salesperson: { id: string; email: string; password: string }
  reviewer: { id: string; email: string; password: string }
  customerId: string
  countryId: string
  pricingEntryIds: { application: string; agent: string }
  quotationId?: string | null
}

export function bootstrapAdminCreds() {
  return {
    email: 'admin@example.com',
    password: 'change-me-on-first-login',
  }
}

export function salespersonEmail(suffix: string) {
  return `salesperson-${suffix}@example.com`
}
export function reviewerEmail(suffix: string) {
  return `reviewer-${suffix}@example.com`
}
export const TEST_PASSWORD = 'e2e-pass-word!!' // >= 8 chars — admin create validator requires it

export function testCustomerPayload(suffix: string) {
  return {
    name: `客户-${suffix} 有限公司`,
    is_returning: false,
    price_sensitive: false,
  }
}

export function testPricingPayload(
  countryId: string,
  suffix: string,
  feeItem: 'application' | 'agent',
  amountCents: number,
) {
  return {
    country_id: countryId,
    service_tier: 'basic',
    fee_item: `e2e-${feeItem}-${suffix}`,
    amount_cny_cents: amountCents,
    effective_from: '2020-01-01', // far enough in the past to always be active
  }
}

export function writeState(s: TestState): void {
  writeFileSync(STATE_PATH, JSON.stringify(s, null, 2), 'utf8')
}

export function readState(): TestState {
  if (!existsSync(STATE_PATH)) {
    throw new Error(
      `state/test-run.json not found at ${STATE_PATH}. Run 01-admin-setup spec first.`,
    )
  }
  return JSON.parse(readFileSync(STATE_PATH, 'utf8')) as TestState
}

export function patchState(patch: Partial<TestState>): TestState {
  const cur = readState()
  const next = { ...cur, ...patch }
  writeState(next)
  return next
}
```

**关键点：**
- `__dirname` 在 ts-node 风格下能工作，Playwright 自带的 ts 编译器处理 CJS-style `__dirname`；验证时用 `console.log` 打印过一次即可
- `TEST_PASSWORD` 长度 ≥ 8 对齐 `apps/api/internal/auth/dto.go:39` 的 `binding:"required,min=8"`
- `effective_from: '2020-01-01'` 保证 pricing 永远激活（见 `pricing.Repository.ListActive`）

- [ ] **Step 2: 创建 `packages/e2e/fixtures/api-client.ts`**

```ts
import { APIRequestContext, expect } from '@playwright/test'

const API_BASE = 'http://localhost:8080/api/v1'

// Cookie names MUST match apps/api/internal/auth/middleware.go constants.
const CSRF_COOKIE = 'tm_csrf_token'
const CSRF_HEADER = 'X-CSRF-Token'

export interface LoggedIn {
  /** APIRequestContext with cookies seeded + CSRF header baked in. */
  request: APIRequestContext
  userId: string
  role: string
}

/**
 * Log in as the given user and return a NEW APIRequestContext that carries
 * the auth + CSRF cookies, with X-CSRF-Token baked into extraHTTPHeaders.
 * Use the returned `request` for all subsequent admin-authenticated calls.
 *
 * Why a new context: Playwright's global `request` fixture shares cookies
 * across tests unless you scope it. Creating a fresh one per login keeps
 * state clean and lets us bake the CSRF header once.
 */
export async function login(
  baseRequest: APIRequestContext,
  email: string,
  password: string,
): Promise<LoggedIn> {
  // Step 1: POST /auth/login on the shared context to set cookies on its jar.
  const resp = await baseRequest.post(`${API_BASE}/auth/login`, {
    data: { email, password },
  })
  expect(
    resp.ok(),
    `login failed: ${resp.status()} ${await resp.text()}`,
  ).toBeTruthy()
  const body = (await resp.json()) as { user: { id: string; role: string } }

  // Step 2: read CSRF cookie from the jar.
  const cookies = await baseRequest.storageState()
  const csrf = cookies.cookies.find((c) => c.name === CSRF_COOKIE)?.value
  expect(csrf, 'CSRF cookie missing after login').toBeTruthy()

  // Step 3: return the existing context (cookies already carried) with header.
  // We piggy-back: APIRequestContext reuses its jar across requests, so
  // baseRequest now has auth cookies; for CSRF we pass the header per-call
  // via the wrapper below.
  return {
    request: createCsrfScopedRequest(baseRequest, csrf!),
    userId: body.user.id,
    role: body.user.role,
  }
}

/**
 * Wrap APIRequestContext so that every non-GET request auto-includes
 * X-CSRF-Token. GET/HEAD/OPTIONS are untouched.
 */
function createCsrfScopedRequest(
  base: APIRequestContext,
  csrfToken: string,
): APIRequestContext {
  const withHeader = <T extends { headers?: Record<string, string> } | undefined>(
    opts: T,
  ) =>
    ({
      ...(opts ?? {}),
      headers: { ...(opts?.headers ?? {}), [CSRF_HEADER]: csrfToken },
    }) as T

  // Proxy: keep identity for GET-style methods, wrap mutating ones.
  return new Proxy(base, {
    get(target, prop, receiver) {
      if (prop === 'post' || prop === 'patch' || prop === 'delete' || prop === 'put') {
        return (url: string, opts?: Parameters<APIRequestContext['post']>[1]) =>
          (target[prop as 'post'] as APIRequestContext['post'])(url, withHeader(opts))
      }
      return Reflect.get(target, prop, receiver)
    },
  }) as APIRequestContext
}

/** POST /admin/users — requires role=admin. */
export async function createUser(
  req: APIRequestContext,
  args: { name: string; email: string; role: 'salesperson' | 'reviewer'; password: string },
): Promise<{ id: string; email: string }> {
  const r = await req.post(`${API_BASE}/admin/users`, { data: args })
  expect(r.ok(), `createUser failed: ${r.status()} ${await r.text()}`).toBeTruthy()
  const body = (await r.json()) as { user: { id: string; email: string } }
  return body.user
}

/** GET /catalog/countries — public to any authenticated user. */
export async function listCountries(
  req: APIRequestContext,
): Promise<{ id: string; code: string; name_zh: string }[]> {
  const r = await req.get(`${API_BASE}/catalog/countries`)
  expect(r.ok(), `listCountries failed: ${r.status()}`).toBeTruthy()
  const body = (await r.json()) as { items: { id: string; code: string; name_zh: string }[] }
  return body.items
}

/** POST /pricing-entries — requires role=admin. */
export async function createPricingEntry(
  req: APIRequestContext,
  args: {
    country_id: string
    service_tier: string
    fee_item: string
    amount_cny_cents: number
    effective_from: string
  },
): Promise<{ id: string }> {
  const r = await req.post(`${API_BASE}/pricing-entries`, { data: args })
  expect(
    r.ok(),
    `createPricingEntry failed: ${r.status()} ${await r.text()}`,
  ).toBeTruthy()
  return (await r.json()) as { id: string }
}

/** POST /customers — reviewer/admin can write; salesperson can too (see router). */
export async function createCustomer(
  req: APIRequestContext,
  args: { name: string; is_returning: boolean; price_sensitive: boolean },
): Promise<{ id: string }> {
  const r = await req.post(`${API_BASE}/customers`, { data: args })
  expect(r.ok(), `createCustomer failed: ${r.status()} ${await r.text()}`).toBeTruthy()
  return (await r.json()) as { id: string }
}
```

**关键点：**
- Cookie + CSRF 的 double-submit 完全由 Proxy 自动处理，调用方不用关心
- `expect()` 在断言失败时 dump 响应体，定位失败快
- country list 走 `/catalog/countries`，这是 M0 seeder 已插入的列表（我们随便挑一个用）

- [ ] **Step 3: 快速验证 fixture 能编译**

```bash
pnpm -C packages/e2e exec tsc --noEmit
```

预期：无类型错误。如果 `__dirname` 报错，改为：
```ts
import { fileURLToPath } from 'node:url'
const __dirname = path.dirname(fileURLToPath(import.meta.url))
```
但 Playwright 默认把 ts config 编成 CJS，`__dirname` 通常可用。先试直接写。

- [ ] **Step 4: Commit**

```bash
git add packages/e2e/fixtures
git commit -m "$(cat <<'EOF'
feat(e2e): add api-client + test-data fixtures (CSRF-aware, state file helpers)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Page Objects

**Files:**
- Create: `packages/e2e/fixtures/pages/login.page.ts`
- Create: `packages/e2e/fixtures/pages/list.page.ts`
- Create: `packages/e2e/fixtures/pages/wizard.page.ts`
- Create: `packages/e2e/fixtures/pages/detail.page.ts`

- [ ] **Step 1: 创建 `packages/e2e/fixtures/pages/login.page.ts`**

```ts
import { Page, expect } from '@playwright/test'

export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/sign-in')
    await expect(this.page.getByRole('button', { name: '登录' })).toBeVisible()
  }

  async signIn(email: string, password: string) {
    await this.page.getByLabel('邮箱').fill(email)
    // PasswordInput renders an <input type="password"> with the 密码 label.
    await this.page.getByLabel('密码').fill(password)
    await this.page.getByRole('button', { name: '登录' }).click()
    // Sign-in route redirects to "/" (safeRedirect default) on success.
    await this.page.waitForURL((url) => !url.pathname.startsWith('/sign-in'), {
      timeout: 10_000,
    })
  }
}
```

**锚点来源：** `apps/web/src/features/auth/sign-in/components/user-auth-form.tsx:95,108,121` — FormLabel `邮箱`/`密码`, Button label `登录`。`safeRedirect` 在 `user-auth-form.tsx:36-41` 默认 `/`。

- [ ] **Step 2: 创建 `packages/e2e/fixtures/pages/list.page.ts`**

```ts
import { Page, expect } from '@playwright/test'

export class ListPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/quotations')
    await expect(this.page.getByRole('heading', { name: '报价列表' })).toBeVisible()
  }

  async clickNew() {
    await this.page.getByRole('link', { name: '新建报价' }).click()
    await this.page.waitForURL(/\/quotations\/new$/)
  }
}
```

**锚点来源：** `apps/web/src/features/quotation/index.tsx:60,62` — h2 `报价列表`, Link `新建报价`。

- [ ] **Step 3: 创建 `packages/e2e/fixtures/pages/wizard.page.ts`**

```ts
import { Page, expect } from '@playwright/test'

export class WizardPage {
  constructor(private page: Page) {}

  async expectStep(n: 1 | 2 | 3 | 4 | 5) {
    await expect(
      this.page.getByText(new RegExp(`第\\s*${n}\\s*步`)),
    ).toBeVisible()
  }

  /**
   * Shadcn Select is a Radix Popover, not a native <select>. We click the
   * trigger to open, then click the option by its visible text.
   */
  async selectCustomerByName(name: string) {
    await this.page.locator('#wizard-customer').click()
    await this.page.getByRole('option', { name }).click()
  }

  async selectCountryByCode(code: string) {
    await this.page.locator('#wizard-country').click()
    // Options render as "中国（CN）" — we match by the code in parens.
    await this.page.getByRole('option', { name: new RegExp(`（${code}）`) }).click()
  }

  async selectTier(tier: 'basic' | 'standard' | 'premium') {
    // RadioGroupItem has id `tier-${value}`. Label wraps it so clicking the
    // option text also works; we target the radio directly for reliability.
    await this.page.locator(`#tier-${tier}`).click()
  }

  async fillNotes(notes: string) {
    await this.page.locator('#wizard-notes').fill(notes)
  }

  async next() {
    await this.page.getByRole('button', { name: '下一步' }).click()
  }

  async back() {
    await this.page.getByRole('button', { name: '上一步' }).click()
  }

  /** On preview step: click 保存并提交 and wait for redirect to /quotations/<id>. */
  async saveAndSubmit(): Promise<string> {
    await this.page.getByRole('button', { name: '保存并提交' }).click()
    await this.page.waitForURL(/\/quotations\/[0-9a-f-]{36}$/, { timeout: 15_000 })
    const m = this.page.url().match(/\/quotations\/([0-9a-f-]{36})$/)
    expect(m, 'could not extract quotation id from URL').toBeTruthy()
    return m![1]
  }

  /**
   * Preview step renders total + signature. Wait for its appearance to be
   * sure the preview mutation finished.
   */
  async waitForPreview() {
    await expect(this.page.getByText('明细 / Line items')).toBeVisible({
      timeout: 10_000,
    })
    // Button becomes enabled only after preview succeeds.
    await expect(this.page.getByRole('button', { name: '保存并提交' })).toBeEnabled()
  }
}
```

**锚点来源：**
- `apps/web/src/features/quotation/wizard/quotation-wizard.tsx:93-95` — `第 X 步`
- `apps/web/src/features/quotation/wizard/steps/step-customer.tsx:23` — id=`wizard-customer`
- `apps/web/src/features/quotation/wizard/steps/step-country.tsx:20` — id=`wizard-country`, 选项文案 `{name_zh}（{code}）`
- `apps/web/src/features/quotation/wizard/steps/step-tier.tsx:30` — id=`tier-basic`/`tier-standard`/`tier-premium`
- `apps/web/src/features/quotation/wizard/steps/step-notes.tsx:14` — id=`wizard-notes`
- `apps/web/src/features/quotation/wizard/steps/step-preview.tsx:122,144` — `明细 / Line items`, Button `保存并提交`
- `quotation-wizard.tsx:113-115` — Button `下一步`

- [ ] **Step 4: 创建 `packages/e2e/fixtures/pages/detail.page.ts`**

```ts
import { Page, Request, expect } from '@playwright/test'

export class DetailPage {
  constructor(private page: Page) {}

  async goto(id: string) {
    await this.page.goto(`/quotations/${id}`)
    await expect(this.page.getByText('状态变更')).toBeVisible()
  }

  async expectStatusBadge(label: '草稿' | '已提交' | '已通过' | '已驳回' | '已取消') {
    // QuotationStatusBadge renders the Chinese label as its text.
    await expect(this.page.getByText(label, { exact: true }).first()).toBeVisible()
  }

  /** Open 调价 sheet, replace first row's amount, save. */
  async adjustFirstLineAmount(newAmountCents: number, comment?: string) {
    await this.page.getByRole('button', { name: '调价' }).click()
    // Sheet title acts as open confirmation.
    await expect(
      this.page.getByRole('heading', { name: '调价' }),
    ).toBeVisible()
    // First numeric input in the sheet is the first line's amount.
    const amountInputs = this.page.locator('input[type="number"]')
    await amountInputs.first().fill(String(newAmountCents))
    if (comment) {
      await this.page.locator('#adjust-comment').fill(comment)
    }
    // Sheet 的确认按钮文案是 "保存"。
    await this.page.getByRole('button', { name: '保存', exact: true }).click()
    // Wait for sheet to close: title gone.
    await expect(
      this.page.getByRole('heading', { name: '调价' }),
    ).toBeHidden({ timeout: 10_000 })
  }

  /** Click 通过 → confirm dialog → 确认. */
  async approve(comment?: string) {
    await this.page.getByRole('button', { name: '通过', exact: true }).click()
    await expect(
      this.page.getByRole('heading', { name: '确认通过' }),
    ).toBeVisible()
    if (comment) {
      await this.page.locator('#comment').fill(comment)
    }
    await this.page.getByRole('button', { name: '确认', exact: true }).click()
    await expect(
      this.page.getByRole('heading', { name: '确认通过' }),
    ).toBeHidden({ timeout: 10_000 })
  }

  /**
   * Click Export PDF dropdown → 中英双语 item, and return the Promise that
   * resolves when the POST /quotations/:id/export request fires.
   *
   * We don't wait for the download itself — `window.open()` in a new tab
   * doesn't necessarily trigger a `download` event, and the spec's intent
   * is to verify the signed URL is generated server-side.
   */
  async triggerBilingualPdfExport(): Promise<Request> {
    // Set up request listener BEFORE clicking.
    const reqPromise = this.page.waitForRequest(
      (req) =>
        req.url().includes('/quotations/') &&
        req.url().endsWith('/export') &&
        req.method() === 'POST',
      { timeout: 15_000 },
    )
    await this.page
      .getByRole('button', { name: /导出 PDF/ })
      .click()
    await this.page.getByRole('menuitem', { name: /中英双语/ }).click()
    return await reqPromise
  }

  /**
   * After `triggerBilingualPdfExport`, wait for the response and verify
   * download_url carries a signed token query parameter.
   */
  async assertExportResponseHasSignedDownloadUrl() {
    const resp = await this.page.waitForResponse(
      (r) =>
        r.url().includes('/quotations/') &&
        r.url().endsWith('/export') &&
        r.request().method() === 'POST',
      { timeout: 15_000 },
    )
    expect(resp.ok(), `export POST not ok: ${resp.status()}`).toBeTruthy()
    const body = (await resp.json()) as { download_url: string }
    expect(body.download_url).toMatch(
      /\/api\/v1\/exports\/[0-9a-f-]{36}\/download\?token=/,
    )
  }
}
```

**锚点来源：**
- `apps/web/src/features/quotation/detail.tsx:70` — `状态变更`
- `apps/web/src/features/quotation/components/quotation-status-badge.tsx` + `types.ts:5-11` — 中文状态文案
- `apps/web/src/features/quotation/components/quotation-action-bar.tsx:98,103,123-124,135,136` — `调价`/`通过`/`确认通过`/`确认`, input id=`comment`
- `apps/web/src/features/quotation/components/quotation-adjust-sheet.tsx:71,119,134` — Sheet title `调价`, textarea id=`adjust-comment`, 按钮 `保存`
- `apps/web/src/features/quotation/components/quotation-export-actions.tsx:35,40,43,46` — Button `导出 PDF`, MenuItem `中英双语`
- `apps/web/src/features/quotation/hooks/use-export.ts:11,18` — POST `/quotations/:id/export` 返 `download_url`
- `apps/api/internal/export/handler.go:291` — download URL 形如 `/api/v1/exports/<uuid>/download?token=<jwt>`

- [ ] **Step 5: 快速验证 page objects 编译**

```bash
pnpm -C packages/e2e exec tsc --noEmit
```

预期：无类型错误。

- [ ] **Step 6: Commit**

```bash
git add packages/e2e/fixtures/pages
git commit -m "$(cat <<'EOF'
feat(e2e): add Login/List/Wizard/Detail page objects

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 01-admin-setup spec (bootstrap data seeding)

**Files:**
- Create: `packages/e2e/tests/01-admin-setup.spec.ts`

- [ ] **Step 1: 创建 `packages/e2e/tests/01-admin-setup.spec.ts`**

```ts
import { test, expect, request as pwRequest } from '@playwright/test'
import {
  bootstrapAdminCreds,
  randomHex,
  reviewerEmail,
  salespersonEmail,
  testCustomerPayload,
  testPricingPayload,
  TEST_PASSWORD,
  writeState,
} from '../fixtures/test-data'
import {
  createCustomer,
  createPricingEntry,
  createUser,
  listCountries,
  login,
} from '../fixtures/api-client'

test.describe.configure({ mode: 'serial' })

test('01-admin-setup: bootstrap admin seeds 2 users + 2 pricing + 1 customer', async () => {
  const suffix = randomHex(4)

  // Dedicated request context for this spec — independent cookie jar.
  const base = await pwRequest.newContext()
  const admin = bootstrapAdminCreds()
  const adminSession = await login(base, admin.email, admin.password)

  // 1. Create salesperson + reviewer.
  const salesperson = await createUser(adminSession.request, {
    name: `E2E Salesperson ${suffix}`,
    email: salespersonEmail(suffix),
    role: 'salesperson',
    password: TEST_PASSWORD,
  })
  const reviewer = await createUser(adminSession.request, {
    name: `E2E Reviewer ${suffix}`,
    email: reviewerEmail(suffix),
    role: 'reviewer',
    password: TEST_PASSWORD,
  })

  // 2. Pick any seeded country (seeder inserts ~60).
  const countries = await listCountries(adminSession.request)
  expect(countries.length, 'expected >=1 country from seeder').toBeGreaterThan(0)
  const country = countries[0]

  // 3. Two pricing entries (basic tier, application + agent fees).
  const applicationEntry = await createPricingEntry(
    adminSession.request,
    testPricingPayload(country.id, suffix, 'application', 100_000), // ¥1000
  )
  const agentEntry = await createPricingEntry(
    adminSession.request,
    testPricingPayload(country.id, suffix, 'agent', 50_000), // ¥500
  )

  // 4. Customer.
  const customer = await createCustomer(
    adminSession.request,
    testCustomerPayload(suffix),
  )

  // 5. Write state for downstream specs.
  writeState({
    suffix,
    adminEmail: admin.email,
    adminPassword: admin.password,
    salesperson: {
      id: salesperson.id,
      email: salesperson.email,
      password: TEST_PASSWORD,
    },
    reviewer: {
      id: reviewer.id,
      email: reviewer.email,
      password: TEST_PASSWORD,
    },
    customerId: customer.id,
    countryId: country.id,
    pricingEntryIds: {
      application: applicationEntry.id,
      agent: agentEntry.id,
    },
    quotationId: null,
  })

  await base.dispose()
})
```

**关键点：**
- 用 `pwRequest.newContext()` 而不是 `test` fixture 里的 `request` — 避免污染后续 spec 的全局 jar
- `suffix = randomHex(4)` 得 8 hex 字符
- `countries[0]` 任挑一个国家；seeder 插入 60 多个（`apps/api/pkg/seeder/seeder.go`）
- `application` 和 `agent` 同 `country_id + basic tier`，`fee_item` 不同才不会互相 replace（pricing 的唯一键是 `country+tier+fee_item`，见 M0）
- 最后调 `writeState` 落地到 `packages/e2e/state/test-run.json`

- [ ] **Step 2: 跑这一个 spec 验证**

前提：`docker compose up -d` 已起。

```bash
pnpm -C packages/e2e exec playwright test 01-admin-setup --reporter=list
```

预期：`1 passed`。并生成 `packages/e2e/state/test-run.json`，内容符合 `TestState` 形状。

手工校验：

```bash
cat packages/e2e/state/test-run.json
```

应该看到：
```json
{
  "suffix": "<8hex>",
  "adminEmail": "admin@example.com",
  ...
  "pricingEntryIds": { "application": "<uuid>", "agent": "<uuid>" },
  "quotationId": null
}
```

如失败：看 `packages/e2e/playwright-report/` 打开排查。

- [ ] **Step 3: Commit**

```bash
git add packages/e2e/tests/01-admin-setup.spec.ts
git commit -m "$(cat <<'EOF'
feat(e2e): add 01-admin-setup spec (admin API creates users/pricing/customer)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 02-salesperson-journey spec (wizard 5 steps + submit)

**Files:**
- Create: `packages/e2e/tests/02-salesperson-journey.spec.ts`

- [ ] **Step 1: 创建 `packages/e2e/tests/02-salesperson-journey.spec.ts`**

```ts
import { test, expect } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { ListPage } from '../fixtures/pages/list.page'
import { WizardPage } from '../fixtures/pages/wizard.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { patchState, readState } from '../fixtures/test-data'
import { listCountries, login } from '../fixtures/api-client'
import { request as pwRequest } from '@playwright/test'

test.describe.configure({ mode: 'serial' })

test('02-salesperson-journey: UI login + 5-step wizard + submit', async ({ page }) => {
  const state = readState()

  // We need the country code (CN/US/...) for the country-selector option
  // label. Do a quick API lookup rather than persisting it in state.
  const base = await pwRequest.newContext()
  const session = await login(base, state.salesperson.email, state.salesperson.password)
  const countries = await listCountries(session.request)
  const country = countries.find((c) => c.id === state.countryId)
  expect(country, 'seeded country not found via API').toBeTruthy()
  await base.dispose()

  // --- UI ---
  const customerName = `客户-${state.suffix} 有限公司`

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.salesperson.email, state.salesperson.password)

  const list = new ListPage(page)
  await list.goto()
  await list.clickNew()

  const wizard = new WizardPage(page)

  // Step 1: 客户
  await wizard.expectStep(1)
  await wizard.selectCustomerByName(customerName)
  await wizard.next()

  // Step 2: 国家
  await wizard.expectStep(2)
  await wizard.selectCountryByCode(country!.code)
  await wizard.next()

  // Step 3: 级别
  await wizard.expectStep(3)
  await wizard.selectTier('basic')
  await wizard.next()

  // Step 4: 备注
  await wizard.expectStep(4)
  await wizard.fillNotes(`E2E run ${state.suffix}`)
  await wizard.next()

  // Step 5: 预览 + 提交
  await wizard.expectStep(5)
  await wizard.waitForPreview()
  const quotationId = await wizard.saveAndSubmit()

  // Verify detail page shows 已提交 badge.
  const detail = new DetailPage(page)
  await expect(page).toHaveURL(new RegExp(`/quotations/${quotationId}$`))
  await detail.expectStatusBadge('已提交')

  patchState({ quotationId })
})
```

**关键点：**
- 先用 API 反查 country 的 `code` —— 因为 UI 的 Country Select 选项文本是 `{name_zh}（{code}）`，按 code 匹配比按 name 更稳（name 有变体）
- `test('...', async ({ page }) => {})` 用 Playwright 自动注入的 `page` fixture；它自带独立 browser context，cookie 不会和前一个 spec 的 API context 冲突
- `expect(page).toHaveURL(...)` 双保险，跟 `saveAndSubmit()` 里的 `waitForURL` 对齐

- [ ] **Step 2: 端到端跑 01 + 02**

```bash
pnpm -C packages/e2e exec playwright test 01-admin-setup 02-salesperson-journey --reporter=list
```

预期：`2 passed`。`state/test-run.json` 的 `quotationId` 不再是 null。

- [ ] **Step 3: Commit**

```bash
git add packages/e2e/tests/02-salesperson-journey.spec.ts
git commit -m "$(cat <<'EOF'
feat(e2e): add 02-salesperson-journey spec (wizard 5 steps + submit)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 03-reviewer-journey spec (adjust + approve)

**Files:**
- Create: `packages/e2e/tests/03-reviewer-journey.spec.ts`

- [ ] **Step 1: 创建 `packages/e2e/tests/03-reviewer-journey.spec.ts`**

```ts
import { test, expect } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { readState } from '../fixtures/test-data'

test.describe.configure({ mode: 'serial' })

test('03-reviewer-journey: UI login + adjust pricing + approve', async ({ page }) => {
  const state = readState()
  expect(state.quotationId, 'quotationId missing — run 02-salesperson-journey first').toBeTruthy()

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.reviewer.email, state.reviewer.password)

  const detail = new DetailPage(page)
  await detail.goto(state.quotationId!)
  await detail.expectStatusBadge('已提交')

  // Adjust: bump first line from ¥1000 (100000 cents) to ¥1500 (150000 cents).
  await detail.adjustFirstLineAmount(150_000, `E2E adjust ${state.suffix}`)

  // After adjust the quotation is still "已提交" — adjust writes a diff
  // into history but doesn't change status. Verify the diff surfaced in
  // the timeline (re-query the status card).
  await detail.expectStatusBadge('已提交')

  // Approve.
  await detail.approve(`E2E approve ${state.suffix}`)

  await detail.expectStatusBadge('已通过')
})
```

**关键点：**
- Adjust 不改 status（参见 `apps/api/internal/quotation/service.go` 的 Adjust 方法），所以调价后还是 `已提交`
- 调价金额用 cents 整数（`adjust-sheet.tsx:92` 的 `Number(e.target.value) || 0`，即 input 里的值就是 cents）
- 批准后应该看到 `已通过`

- [ ] **Step 2: 跑 01 + 02 + 03 三联**

```bash
pnpm -C packages/e2e exec playwright test 01-admin-setup 02-salesperson-journey 03-reviewer-journey --reporter=list
```

预期：`3 passed`。

- [ ] **Step 3: Commit**

```bash
git add packages/e2e/tests/03-reviewer-journey.spec.ts
git commit -m "$(cat <<'EOF'
feat(e2e): add 03-reviewer-journey spec (adjust + approve)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 04-export spec (PDF bilingual download)

**Files:**
- Create: `packages/e2e/tests/04-export.spec.ts`

- [ ] **Step 1: 创建 `packages/e2e/tests/04-export.spec.ts`**

```ts
import { test, expect } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { readState } from '../fixtures/test-data'

test.describe.configure({ mode: 'serial' })

test('04-export: PDF bilingual export fires signed download URL', async ({ page }) => {
  const state = readState()
  expect(state.quotationId, 'quotationId missing').toBeTruthy()

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.salesperson.email, state.salesperson.password)

  const detail = new DetailPage(page)
  await detail.goto(state.quotationId!)
  // Must be approved — 03-reviewer-journey put it into 已通过.
  await detail.expectStatusBadge('已通过')

  // Set up response listener BEFORE clicking.
  const exportResponsePromise = page.waitForResponse(
    (r) =>
      r.url().includes(`/quotations/${state.quotationId}/export`) &&
      r.request().method() === 'POST',
    { timeout: 15_000 },
  )

  await detail.triggerBilingualPdfExport()

  const resp = await exportResponsePromise
  expect(resp.ok(), `export POST not ok: ${resp.status()}`).toBeTruthy()
  const body = (await resp.json()) as {
    format: string
    language: string
    download_url: string
  }
  expect(body.format).toBe('pdf')
  expect(body.language).toBe('bilingual')
  expect(body.download_url).toMatch(
    /\/api\/v1\/exports\/[0-9a-f-]{36}\/download\?token=.+/,
  )
})
```

**关键点：**
- 必须用 `salesperson` 登录 —— QuotationExportActions 对 approved quotation 显示（`quotation-export-actions.tsx:26`），owner/reviewer/admin 都能看（无显式权限门），但和 03-spec 角色换一下也没坏处
- `waitForResponse` 而非 `waitForRequest`：确保 gotenberg 回来了签名，这才证明 PDF 导出链路完整（如果 gotenberg 挂了 handler 返 5xx）
- 不断言文件真下载 —— `window.open('_blank')` 在 chromium headless 里通常是新 tab 而非下载事件

- [ ] **Step 2: 全套 4 个 spec 跑通**

```bash
pnpm -C packages/e2e test
```

预期：`4 passed`。HTML 报告在 `packages/e2e/playwright-report/index.html`。

- [ ] **Step 3: Commit**

```bash
git add packages/e2e/tests/04-export.spec.ts
git commit -m "$(cat <<'EOF'
feat(e2e): add 04-export spec (PDF bilingual download signed URL)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 收尾 — 端到端稳定性 + 文档

**Files:**
- Modify: `packages/e2e/README.md`（补充 troubleshooting）
- Verify: `make e2e` 跑通 3 次

- [ ] **Step 1: `make e2e` 端到端跑一次**

```bash
docker compose up -d
# 等 20s 让 api 起好
make e2e
```

预期：`4 passed`，总耗时 < 60s。

- [ ] **Step 2: 再跑两次验证稳定性**

```bash
make e2e
make e2e
```

预期：两次都全绿。因为每次生成新 suffix，不冲突。

- [ ] **Step 3: 破坏性验证 1 — stack 未起**

```bash
docker compose stop api
make e2e
```

预期：preflight 失败，输出 `API not responding. Run 'docker compose up -d' first.`，exit code 1。然后：

```bash
docker compose start api
```

- [ ] **Step 4: 破坏性验证 2 — state 缺失**

```bash
rm packages/e2e/state/test-run.json
pnpm -C packages/e2e exec playwright test 02-salesperson-journey --reporter=list
```

预期：02-spec 失败，错误信息包含 `Run 01-admin-setup spec first.`（来自 `test-data.ts` 的 readState）。

- [ ] **Step 5: 恢复 + 完整回归**

```bash
make e2e
```

预期：仍然 4 passed。

- [ ] **Step 6: 补充 README troubleshooting 段落**

在 README.md 末尾追加：

```markdown
## Troubleshooting

### "API not responding" from `make e2e`

Check compose status:

    docker compose ps

If api is unhealthy, tail logs:

    docker compose logs api

### "quotationId missing — run 02-salesperson-journey first"

Spec 03 or 04 executed before 02 succeeded. Start fresh:

    pnpm -C packages/e2e test

### Flaky "timeout waiting for /quotations/<uuid>"

If preview/submit is slow on first run (cold postgres), bump the wizard
timeout in `fixtures/pages/wizard.page.ts` `saveAndSubmit`, or restart
the compose stack with a warm DB.

### Browser binary not found

    pnpm -C packages/e2e install:browsers

### DB gets polluted across runs

Each run generates a fresh 8-hex suffix, so test data never collides. But
rows accumulate. Clean slate:

    docker compose down -v && docker compose up -d
```

- [ ] **Step 7: 更新 Makefile help 已在 T1 做过，确认一下**

```bash
make help | grep e2e
```

预期输出一行：`  e2e           run Playwright E2E (requires docker compose up -d)`

- [ ] **Step 8: Commit 收尾**

```bash
git add packages/e2e/README.md
git commit -m "$(cat <<'EOF'
docs(e2e): add troubleshooting section after 3x green smoke run

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review Checklist

### Spec coverage（对标 `docs/superpowers/specs/2026-04-26-m5-e2e-playwright-design.md`）

- §3.1 包含 `packages/e2e/` workspace 包 → T1 ✅
- `package.json` + `playwright.config.ts` + `@playwright/test` → T1 ✅
- `fixtures/api-client.ts` 薄 fetch 封装 CSRF + cookie → T2 ✅
- `fixtures/test-data.ts` 随机后缀 + 用户凭据 + pricing → T2 ✅
- `fixtures/pages/*` 四个 Page Object → T3 ✅
- 4 个 spec 按前缀数字排序 → T4/T5/T6/T7 ✅
- `state/test-run.json`（gitignored） → T1/T2 ✅
- 根 `Makefile` `e2e` target → T1 ✅
- `packages/e2e/README.md` → T1/T8 ✅
- §3.2 不包含：CI 不接入 ✅（只动 Makefile，没改 `.github/`）
- §4.2 playwright.config 关键设置（fullyParallel/workers/trace/retries）→ T1 ✅
- §4.3 随机后缀策略 → T2 `randomHex(4)` ✅
- §4.4 CSRF+cookie flow → T2 `login()` + Proxy ✅
- §4.5 spec 依赖链 → T4→T5→T6→T7 每个 spec 的顺序对齐 ✅
- §5.1 下载断言用 `waitForResponse`/`waitForRequest` 而不是 download event → T7 ✅
- §5.2 serial + state file 通信 → T1 config + T2 state helpers ✅
- §5.3 CSRF header 从 cookie jar 读 → T2 `createCsrfScopedRequest` ✅
- §5.4 test-run.json shape → T2 `TestState` interface 对齐 ✅
- §6 stack 未启动 / spec 依赖断链 → T1 Makefile preflight + T2 readState 抛错 ✅
- §7 稳定性 10 次 ≥ 9 次全绿、时间 < 60s → T8 验证 3x，10x 不做但可后续确认

### Placeholder scan（红旗搜索）

- 无 `TBD` / `TODO` / `implement later` / `fill in details`
- 无 `add appropriate error handling` / 模糊的 "write tests for the above"
- 无 `Similar to Task N`（每任务代码完整独立）
- 无未定义的类型引用（`TestState`/`LoggedIn`/各 Page Object 都在前序 task 里定义）

### Type consistency

- `TestState` 字段在 `writeState` / `patchState` / `readState` 调用时类型一致
- `createUser` 返回 `{ id, email }`，spec 用 `salesperson.id` / `salesperson.email` 一致
- `listCountries` 返回 `{ id, code, name_zh }`，wizard 用 `code` 匹配一致
- `DetailPage.expectStatusBadge` 接受的 union 与 `QUOTATION_STATUS_LABEL_ZH` 的 value 一致

---

## 执行建议

- **推荐 Subagent-Driven**：每个 task 都独立（文件边界清晰），fresh context + 双阶段 review 能最大保证 TS 类型正确、locator 精准
- **替代 Inline**：如果想一次性跑，用 executing-plans
- **强制前置**：所有 task 都依赖 `docker compose up -d` 已在跑；T4 起每步都需要 stack 活着
