# M5: Playwright E2E 全链路 — 设计规格

> **对标 roadmap**：`docs/superpowers/plans/2026-04-25-mvp-p0-roadmap.md` 的 M5 里程碑
> **前置**：M1（导出 PDF）+ M2（审核流）+ M3（向导前端）均已完成
> **交付 DoD**：#9（端到端验证：登录→客户→向导→提交→审核→调价→通过→下载 PDF）
> **日期**：2026-04-26

## 1. 背景与目标

目前项目有三层测试：
- 后端 Go 单元/集成测试（testcontainers-go + Postgres）
- 前端 vitest-browser 组件测试（MSW mock 后端）
- 前端 vitest-browser 集成测试（M3 的 11 个 wizard 场景）

**没有真实的端到端测试**——没人验证过"docker compose up 之后，业务员能否实际走完一次报价"。M5 的目标是补上这一层：用 Playwright 驱动真实浏览器，打真实的 API，走真实的 gotenberg PDF 导出。

---

## 2. 与 roadmap 的偏差

| roadmap 原文 | 本 M5 决策 | 理由 |
|---|---|---|
| "CI 持续验证完整链路"（DoD #9） | **仅本地跑，CI 暂不接入**（决策 D6） | 用户显式选择；docker-compose 在 GitHub Actions 上起 stack 有 60-90s 额外开销，且 gotenberg/postgres 偶发 flaky；可后续另起小型 milestone 把它加进 CI |
| "登录→客户→向导→提交→审核→调价→通过→下载 PDF" 单链路描述 | **拆为 4 个 spec 按角色分组**（决策 D3） | 调试粒度高；失败定位快；每个 spec 专注一个 actor 的视角；用 Playwright 的 serial 模式 + 共享 state 文件保持前后依赖 |

---

## 3. 范围

### 3.1 包含

- 新 workspace 包 `packages/e2e/`：独立 `package.json` + `playwright.config.ts` + `@playwright/test` 依赖
- 共享 fixtures：
  - `fixtures/api-client.ts`：薄 fetch 封装，处理 CSRF token + cookie 登录
  - `fixtures/test-data.ts`：生成随机后缀、用户凭据、pricing 数据
  - `fixtures/pages/*`：LoginPage / ListPage / WizardPage / DetailPage 四个 page object
- 4 个 spec 文件（按前缀数字排序执行）：
  - `01-admin-setup.spec.ts` → 登录 bootstrap admin + 建用户/pricing/customer
  - `02-salesperson-journey.spec.ts` → 走向导 5 步 + 保存并提交
  - `03-reviewer-journey.spec.ts` → 调价 + 通过审核
  - `04-export.spec.ts` → 下载 PDF（双语）
- state 共享文件 `packages/e2e/state/test-run.json`（gitignore）
- 根 `Makefile` 加 `e2e` target
- `packages/e2e/README.md` 说明如何跑

### 3.2 不包含

- GitHub Actions 集成（DoD #9 要求"CI 持续验证"，本轮仅本地跑；见 §2）
- 并行执行（serial 模式强制 workers=1，避免前后依赖踩踏）
- 多浏览器矩阵（只跑 chromium；firefox/webkit 留给未来）
- 失败场景测试（驳回、撤回、取消）——本轮只走 happy path
- 视觉回归（Percy / Chromatic）
- 负载/性能测试
- 数据库 reset/清理：用随机后缀避免名字冲突（见 §4.3）
- 新增后端 seeder 或 reset 端点：测试全靠真实 admin API 建数据

---

## 4. 架构

### 4.1 目录结构

```
packages/e2e/
├── package.json              # @playwright/test ^1.59, typescript
├── tsconfig.json             # extends 根 tsconfig
├── playwright.config.ts      # baseURL, workers=1, serial, trace, chromium-only
├── .gitignore                # state/, test-results/, playwright-report/
├── fixtures/
│   ├── api-client.ts         # loginAsBootstrapAdmin / postJSON / 读 CSRF cookie
│   ├── test-data.ts          # randSuffix(), testUsers(), testPricing(), testCustomer()
│   └── pages/
│       ├── login.page.ts     # goto('/sign-in') + fill + 提交 + 等待跳转
│       ├── list.page.ts      # 点击"新建报价"、定位行
│       ├── wizard.page.ts    # step 1-5 的点击链 + "保存并提交"
│       └── detail.page.ts    # status badge + 调价面板 + 通过按钮 + 导出下拉
├── tests/
│   ├── 01-admin-setup.spec.ts
│   ├── 02-salesperson-journey.spec.ts
│   ├── 03-reviewer-journey.spec.ts
│   └── 04-export.spec.ts
├── state/                    # gitignored
│   └── test-run.json
└── README.md
```

### 4.2 `playwright.config.ts` 关键设置

```ts
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,      // 前后依赖,必须串行
  workers: 1,
  retries: 0,                // CI 不接入,本地跑失败直接看报告
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry', // 保险;手动 rerun 时能抓 trace
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  reporter: [['list'], ['html', { outputFolder: 'playwright-report' }]],
})
```

stack 起好的责任在用户：跑 `make e2e` 前必须先 `docker compose up -d` 起完 api+web+postgres+gotenberg。`Makefile` 的 e2e target 会在启动 playwright 前打一个 curl 到 `http://localhost:8080/health` 检查后端就绪。

### 4.3 随机后缀策略（数据隔离）

每次 `01-admin-setup.spec` 开头生成 `suffix = randomHex(8)`（例 `ab12cd34`），后续所有测试数据命名都带上这个后缀：
- 用户：`salesperson-ab12cd34@example.com` / `reviewer-ab12cd34@example.com`
- 用户名：`E2E Salesperson ab12cd34`
- 客户：`客户-ab12cd34 有限公司`
- pricing：country 用任何已有国家（seeder 已插入），fee_item 用 `e2e-application-ab12cd34` + `e2e-agent-ab12cd34`

后缀写入 `state/test-run.json`，后续 spec 读取。多次 run 数据会累积在 DB 里，但互不冲突。

### 4.4 CSRF + 登录 flow

现有 `auth.RequireAuth` 中间件验证 Cookie JWT；`auth.CSRF` 中间件对非 GET 要求 `X-CSRF-Token` 头匹配 cookie 中的 `csrf_token`。Playwright UI 测试走真实浏览器表单，cookie + CSRF 自动由浏览器管理。

**但 api-client fixture 需要直接发 admin API 请求**（建用户/pricing），它必须手动处理：
1. POST `/api/v1/auth/login` → 得到 `tm_access_token` + `tm_refresh_token` + `tm_csrf_token` cookie（后者 **not httpOnly**，可被前端读取）
2. 后续 POST/PATCH/DELETE 请求带 `X-CSRF-Token` 头 = `tm_csrf_token` cookie 值

api-client 用 Playwright 的 `APIRequestContext`（`request` fixture），它自动保持 cookie jar 并支持 `extraHTTPHeaders`。

### 4.5 spec 依赖链

```
01-admin-setup.spec:
  ├─ 生成 suffix,写 state/test-run.json
  ├─ APIRequestContext 登录 bootstrap admin
  ├─ POST /admin/users × 2 (salesperson + reviewer)
  ├─ POST /pricing-entries × 2
  ├─ POST /customers × 1
  └─ 写 state: { suffix, salesperson, reviewer, customerId, countryId, pricingEntryIds }

02-salesperson-journey.spec:
  ├─ 读 state
  ├─ UI 登录 salesperson → /sign-in → 等待跳 /
  ├─ goto '/quotations/new'
  ├─ 走 5 步(customer 选 state.customerId、country 任选、tier=basic、备注、预览)
  ├─ 点"保存并提交"
  ├─ 等待跳 /quotations/<id> + "已提交" badge
  └─ 从 URL 解析 quotationId,追加进 state

03-reviewer-journey.spec:
  ├─ 读 state (含 quotationId)
  ├─ UI 登录 reviewer
  ├─ goto '/quotations/<id>'
  ├─ 点"调价" → Sheet 打开 → 改 application 行金额 10000 → 15000 → 保存
  ├─ 验证 diff 行出现在 history
  ├─ 点"通过" → 确认
  └─ 断言"已通过" badge

04-export.spec:
  ├─ 读 state
  ├─ UI 登录 salesperson
  ├─ goto '/quotations/<id>'
  ├─ 点"导出 PDF"下拉 → 点"中英双语"
  ├─ 等待 page.waitForEvent('popup') 或 waitForRequest(/exports\/.+\/download/)
  └─ 断言请求 URL 含 download_url 签名参数(token=)
```

---

## 5. 关键实现决策

### 5.1 下载断言方式

M1 的导出 action 实际 JS 行为：`window.open(DOWNLOAD_URL, '_blank', 'noopener')`。在 Playwright 里这会触发 `page.on('popup')` 事件，但不真的下载文件（`_blank` + 未设置 Accept header 只是打开一个 tab）。

**推荐**：用 `page.waitForRequest(url => url.includes('/exports/') && url.includes('/download'))` 断言**请求发出**，不等请求完成。这样不依赖 gotenberg 真能生成 PDF（间接也验证了——如果 gotenberg 挂，就不会拿到 download_url）。

### 5.2 测试间依赖

`playwright.config.ts` 设置 `fullyParallel: false` + `workers: 1`。tests 按文件名字母序执行（01-、02-、03-、04-前缀）。state 文件作为跨 spec 的通信介质，`fs.writeFileSync` + `fs.readFileSync`（同步简单，无并发）。

### 5.3 CSRF token 获取

Playwright 的 `APIRequestContext.storageState()` 能导出 cookie，但我们只需单个 cookie 值。用 `context.cookies('http://localhost:8080')` 拿到所有 cookie，过滤出 `tm_csrf_token`，设置到后续请求的 `X-CSRF-Token` header。

### 5.4 test-run.json shape

```json
{
  "suffix": "ab12cd34",
  "adminEmail": "admin@example.com",
  "adminPassword": "change-me-on-first-login",
  "salesperson": {
    "id": "uuid",
    "email": "salesperson-ab12cd34@example.com",
    "password": "e2e-pw!!"
  },
  "reviewer": {
    "id": "uuid",
    "email": "reviewer-ab12cd34@example.com",
    "password": "e2e-pw!!"
  },
  "customerId": "uuid",
  "countryId": "uuid",
  "pricingEntryIds": ["uuid-app", "uuid-agent"],
  "quotationId": null // 02-spec 会写入
}
```

---

## 6. 错误处理与边界

- **stack 未启动**：Makefile `e2e` target 预检 `curl http://localhost:8080/health` 失败则 exit 1 + 打印 `请先运行 docker compose up -d`
- **admin bootstrap 未启用**：本地 docker-compose.yml 固定设 `BOOTSTRAP_ADMIN_EMAIL/PASSWORD`；不检查
- **spec 依赖断链**（02 跑前 01 没跑）：读 state/test-run.json 失败 → 抛错 "Run 01-admin-setup first"；不做 hidden fallback
- **随机后缀碰撞**：8 hex = 4 billion，生产里每天跑多次完全不会撞
- **API 调用超时**：APIRequestContext 默认 30s timeout，够用；超时就 fail + trace
- **UI 元素定位超时**：Playwright 默认 5s locator timeout，关键步骤（保存并提交后跳详情页）加显式 `waitForURL`
- **PDF 导出失败**（gotenberg 挂）：导出请求返回 5xx → 测试 fail；错误信息里看得到 HTTP 状态
- **前端 build 未刷新**：Makefile `e2e` target 文档里提醒用户 `docker compose up --build`

---

## 7. 测试策略

E2E 本身就是测试。没有"对 E2E 的测试"这一层。只有质量衡量：

- **稳定性目标**：10 次连跑 ≥ 9 次全绿（偶发网络/timing flaky 允许 1 次）
- **时间预算**：完整跑完 4 个 spec 控制在 60 秒以内（stack 启动时间不算）
- **覆盖率衡量**：DoD 清单 #1 / #4 / #5 / #6 / #8 都被本 E2E 触达
- **工具反馈**：`packages/e2e/playwright-report/`（HTML 报告）+ `packages/e2e/test-results/`（失败截图/trace）

---

## 8. 任务拆分（预算 8，实际 8）

| # | 任务 | 文件/范围 |
|---|---|---|
| **T1** | `packages/e2e/` 脚手架 | `package.json`、`tsconfig.json`、`playwright.config.ts`、`.gitignore`、`README.md`；注册到 `pnpm-workspace.yaml`（已含 `packages/*` 通配）；根 `Makefile` 加 `e2e` target |
| **T2** | `fixtures/api-client.ts` + `fixtures/test-data.ts` | login helper、CSRF 解析、random 后缀、state 文件读写 |
| **T3** | `fixtures/pages/*.ts` | LoginPage、ListPage、WizardPage、DetailPage 四个 page object |
| **T4** | `01-admin-setup.spec.ts` | bootstrap admin 登录 → 建 2 user + 2 pricing + 1 customer → 写 state |
| **T5** | `02-salesperson-journey.spec.ts` | UI 登录 salesperson → wizard 5 步 → 保存并提交 → quotationId 写 state |
| **T6** | `03-reviewer-journey.spec.ts` | UI 登录 reviewer → 调价面板 → 保存 → 通过审核 |
| **T7** | `04-export.spec.ts` | UI 登录 salesperson → 导出下拉 → 中英双语 → 断言下载请求发出 |
| **T8** | 收尾 | README 跑法示例 + Makefile 预检逻辑 + 跑一次完整 E2E 验证 |

**执行顺序**：T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8。每步依赖前一步（T3 依赖 T2 的 helpers；T4+ 依赖 T3 的 page objects）。

---

## 9. 自检清单

- [x] 所有偏差（§2）都有理由且与用户对齐（6 次 AskUserQuestion）
- [x] state 文件不入库（`.gitignore` 覆盖 `packages/e2e/state/`）
- [x] serial 执行确保 spec 依赖正确解开
- [x] 随机后缀避免重跑数据冲突，无需 DB reset
- [x] 下载断言用 `waitForRequest`，不依赖真下载文件
- [x] 4 个 spec 覆盖 DoD #1（登录）+ #4（向导）+ #5（审核流）+ #6（导出）+ #8（权限）
- [x] Makefile 预检 stack 就绪，避免误触发导致看不懂的 timeout
- [x] CI 不接入是已知偏差，在 §2 明记；不是遗漏
