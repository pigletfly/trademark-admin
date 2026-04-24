# Frontend Auth Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 shadcn-admin 模板的"假登录 + Zustand 假 token"替换成真实对接 Plan 2 后端的登录流程：Axios + httpOnly cookie + CSRF 双提交 + 401→refresh 自动重试；TanStack Router `beforeLoad` 做路由守卫；Clerk 彻底移除；UI 中文化。

**Architecture:** `src/lib/api.ts` 提供全局 axios 实例（`withCredentials`、CSRF 请求拦截、401 响应拦截单次 refresh 并重放）。`src/stores/auth-store.ts` 只保留 user + status，不再读写 access token（httpOnly cookie 不可见）。`src/features/auth/hooks/` 下三个 hook（useMe/useLogin/useLogout）包装 TanStack Query。`_authenticated/route.tsx` 用 `beforeLoad` 预取 `['me']`，失败 redirect 到 `/sign-in`。Sign-up / Forgot-password / OTP 路由保留为"联系管理员"提示页面（MVP 不自助注册）。

**Tech Stack:** 复用模板：React 18 + TanStack Router + TanStack Query + Zustand + shadcn/ui + axios + Vitest/browser (Playwright chromium) + MSW（新增，用于拦截网络请求做组件集成测试）。

**上下文提示：**
- Plan 2 后端已合回 main：`/api/v1/auth/{login,logout,refresh,me}` 可用；/admin/* 需 admin 角色；bootstrap admin 通过 docker compose 创建。
- Axios 在 `apps/web/package.json` 已经是直接依赖。主入口 `apps/web/src/main.tsx` 已用 axios 错误类型，TanStack Query queryCache.onError 里已经对 401 触发 `authStore.reset()` + 跳转 `/sign-in`，本 plan 会重用并微调这套逻辑。
- 现有 fake sign-in 在 `apps/web/src/features/auth/sign-in/components/user-auth-form.tsx`，用 `toast.promise(sleep(2000))` 模拟异步成功。
- Clerk 集成位于 `apps/web/src/routes/clerk/` **整个目录**，依赖 `@clerk/react ^6.4.2`。模板明确说 "safely remove this directory and @clerk/react"。
- `apps/web/vite.config.ts` 已经配好 `test.browser.provider = playwright()` + chromium；首次跑需要 `pnpm -C apps/web test:browser:install`。
- 中文化：所有 visible 文案改为简体中文；保留英文的仅：品牌名、email 占位符、role code 常量。

**在 `feat/plan-3-frontend-auth` 分支上执行**（从 main 切出）。

---

## 文件结构（本 plan 结束时新增/修改/删除）

```
apps/web/
├── package.json                           (删除 @clerk/react；新增 msw（devDep）)
├── .env.example                           (新增 VITE_API_BASE_URL=/api/v1；删除 VITE_CLERK_PUBLISHABLE_KEY)
├── src/
│   ├── main.tsx                           (微调 queryCache onError：reset 后 navigate 已有，逻辑保留；router context 加 queryClient 已有，沿用)
│   ├── lib/
│   │   ├── api.ts                         (新增：axios instance + CSRF 拦截 + 401 refresh 重试)
│   │   ├── api.test.ts                    (新增：CSRF header、401 单次重试、refresh 失败不死循环)
│   │   └── csrf.ts                        (新增：从 tm_csrf_token cookie 读取 token 的小工具)
│   ├── stores/
│   │   ├── auth-store.ts                  (重写：user + status 两字段，移除 accessToken 与 cookie 读写)
│   │   └── auth-store.test.ts             (重写)
│   ├── features/auth/
│   │   ├── hooks/
│   │   │   ├── use-me.ts                  (新增)
│   │   │   ├── use-login.ts               (新增)
│   │   │   ├── use-logout.ts              (新增)
│   │   │   └── me-query.ts                (新增：meQueryOptions 给 beforeLoad 共用)
│   │   ├── sign-in/
│   │   │   ├── index.tsx                  (改中文 + 移除 Sign Up 链接)
│   │   │   └── components/user-auth-form.tsx  (改：useLogin 替代 sleep + 中文文案 + 删 GitHub/Facebook)
│   │   └── auth-layout.tsx                (无需改，布局复用)
│   ├── routes/
│   │   ├── __root.tsx                     (无需改)
│   │   ├── (auth)/
│   │   │   ├── sign-in.tsx                (保留)
│   │   │   ├── sign-up.tsx                (替换为"请联系管理员"页面)
│   │   │   ├── forgot-password.tsx        (替换为"请联系管理员"页面)
│   │   │   ├── otp.tsx                    (替换为"请联系管理员"页面)
│   │   │   └── sign-in-2.tsx              (保留模板里的备用样式，暂不动)
│   │   ├── _authenticated/
│   │   │   └── route.tsx                  (加 beforeLoad：预取 me，401 redirect 到 /sign-in)
│   │   └── clerk/                         (整个目录删除)
│   └── components/
│       └── layout/
│           └── profile-dropdown.tsx       (如果现有 sign-out 按钮在此，wire 到 useLogout；否则跳过)
└── test/
    └── msw/
        ├── handlers.ts                    (新增：MSW http handlers 模拟 /api/v1/auth/*)
        └── server.ts                      (新增：浏览器 worker setup)
```

---

### Task 1: 移除 Clerk

目标：彻底清理模板里的 Clerk 集成，让前端依赖树干净。

**Files:**
- Delete: `apps/web/src/routes/clerk/` 整个目录
- Modify: `apps/web/package.json`（删 `@clerk/react` 依赖）
- Modify: `apps/web/.env.example`（如有 `VITE_CLERK_PUBLISHABLE_KEY` 删掉）
- Regenerate: `apps/web/src/routeTree.gen.ts`（vite dev/build 会自动更新，不手动改）

- [ ] **Step 1: 删除 clerk 路由目录**

```bash
cd /Users/adam/workspace/github/trademark-admin
rm -rf apps/web/src/routes/clerk
```

- [ ] **Step 2: 删依赖**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web remove @clerk/react
```

Expected: `package.json` 里 `@clerk/react` 消失，`pnpm-lock.yaml` 更新。

- [ ] **Step 3: 清理 .env.example**

Read `apps/web/.env.example`。如果有 `VITE_CLERK_PUBLISHABLE_KEY` 行，删掉。如果整个文件不存在，创建一个：
```
VITE_API_BASE_URL=/api/v1
```

（`VITE_API_BASE_URL` 将给 Task 3 的 axios 用；dev 环境 vite.config 里没配代理到 /api，所以 nginx 反代只在 docker-compose 生效；本 plan 后续步骤会加 vite dev 代理。）

- [ ] **Step 4: 在 vite.config.ts 里加 dev 代理**

Read `apps/web/vite.config.ts`，在 `defineConfig({ ... })` 的 `plugins/resolve/test` 同层加 `server` 配置：

```ts
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
```

这样 `pnpm -C apps/web dev` 会把 `/api/v1/*` 转给本地 8080 跑的 Go 后端。

- [ ] **Step 5: 跑构建 + 测试确认无 Clerk 引用**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web lint
pnpm -C apps/web build
```

Expected: lint 无 Clerk 相关错误；build 成功（routeTree.gen.ts 会自动再生成，不再有 `/clerk` 路由）。

如果 build 报 "Cannot find module '@clerk/react'"，说明还有别处 import，用 `grep -rn "@clerk" apps/web/src/` 找到并删除或替换。

- [ ] **Step 6: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/package.json apps/web/pnpm-lock.yaml apps/web/vite.config.ts apps/web/.env.example apps/web/src/routeTree.gen.ts
git add -u apps/web/src/routes/   # 处理被删目录
git commit -m "$(cat <<'EOF'
chore(web): remove @clerk/react demo integration and routes

Plan 3 uses the self-built cookie-based auth on apps/api. Clerk was a
template demo we never wired up. Drop the dep, delete src/routes/clerk/,
add vite dev proxy to the Go API so /api/v1/* works in pnpm dev.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: 重写 auth store

目标：auth store 只保留 user + status，移除 accessToken 与 cookie 读写（httpOnly cookie 浏览器 JS 读不到，存 store 里是虚假安全感）。

**Files:**
- Rewrite: `apps/web/src/stores/auth-store.ts`
- Rewrite: `apps/web/src/stores/auth-store.test.ts`

- [ ] **Step 1: 写失败测试**

Replace `apps/web/src/stores/auth-store.test.ts` 全文为：
```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore, type AuthUser } from './auth-store'

const sampleUser: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Root Admin',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

describe('auth-store', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
  })

  it('starts with no user and status=unknown', () => {
    const { auth } = useAuthStore.getState()
    expect(auth.user).toBeNull()
    expect(auth.status).toBe('unknown')
  })

  it('setUser transitions to authenticated', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    const next = useAuthStore.getState().auth
    expect(next.user).toEqual(sampleUser)
    expect(next.status).toBe('authenticated')
  })

  it('markUnauthenticated clears user and flips status', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    auth.markUnauthenticated()
    const next = useAuthStore.getState().auth
    expect(next.user).toBeNull()
    expect(next.status).toBe('unauthenticated')
  })

  it('reset returns to unknown', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    auth.reset()
    const next = useAuthStore.getState().auth
    expect(next.user).toBeNull()
    expect(next.status).toBe('unknown')
  })
})
```

- [ ] **Step 2: 跑测试看失败**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/stores/auth-store.test.ts
```
Expected: 失败（AuthUser / status 字段 / markUnauthenticated 不存在）。

- [ ] **Step 3: 重写 auth-store.ts**

Replace `apps/web/src/stores/auth-store.ts` 全文为：
```ts
import { create } from 'zustand'

export type AuthStatus = 'unknown' | 'authenticated' | 'unauthenticated'

export interface AuthUser {
  id: string
  name: string
  email: string
  phone: string
  role: 'salesperson' | 'reviewer' | 'admin'
  status: 'active' | 'disabled'
}

interface AuthState {
  auth: {
    user: AuthUser | null
    status: AuthStatus
    setUser: (user: AuthUser) => void
    markUnauthenticated: () => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    status: 'unknown',
    setUser: (user) =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user, status: 'authenticated' },
      })),
    markUnauthenticated: () =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user: null, status: 'unauthenticated' },
      })),
    reset: () =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user: null, status: 'unknown' },
      })),
  },
}))
```

- [ ] **Step 4: 跑测试看通过**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/stores/auth-store.test.ts
```
Expected: 4 条 PASS。

- [ ] **Step 5: 处理仍 import 旧 API 的调用方**

```bash
cd /Users/adam/workspace/github/trademark-admin
grep -rn "auth.setAccessToken\|auth.resetAccessToken\|auth.accessToken" apps/web/src/
```

现有调用方会有几处。依次处理：
- `apps/web/src/main.tsx` 的 `queryCache.onError` 401 分支：把 `toast.error('Session expired!')` 改成 `toast.error('登录状态已失效')`；把 `useAuthStore.getState().auth.reset()` 改成 `useAuthStore.getState().auth.markUnauthenticated()`（语义：reset 是 unknown，我们已经确定 session 无效就是 unauthenticated）
- `user-auth-form.tsx` 的 `auth.setAccessToken` 将在 Task 5 一并改
- 任何其他处：把 `auth.setAccessToken(...)` 直接删；`auth.accessToken` 读取改为 `auth.status === 'authenticated'`；`auth.setUser(mockUser)` 换成真实 user 时再改

**本 step 只**改能立即修的无副作用调用点；对 user-auth-form.tsx 这种逻辑修改留到 Task 5。

如果 `pnpm -C apps/web build` 报错，确认错误在本 task 不关心的 user-auth-form 上——若是，暂时给该引用加 `// @ts-expect-error — rewired in Task 5` 占位（允许过 ts 检查）。本 step 目标是 store 独立通过。

- [ ] **Step 6: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/stores/auth-store.ts apps/web/src/stores/auth-store.test.ts apps/web/src/main.tsx
git commit -m "$(cat <<'EOF'
refactor(web): auth store holds user + status only, no access token

Access and refresh tokens live in httpOnly cookies managed by apps/api.
Storing them in Zustand/JS was fake safety. Replace accessToken field
with AuthStatus enum ('unknown' | 'authenticated' | 'unauthenticated').
main.tsx queryCache onError now uses markUnauthenticated for the 401
path (accurate) and shows the Chinese session-expired toast.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: API client (axios + CSRF + 401 refresh 重试)

目标：一个模块导出 `api` axios 实例，所有 HTTP 调用走它。请求拦截器把 `tm_csrf_token` cookie 值塞进 `X-CSRF-Token` header；响应拦截器遇到 401 调一次 `/auth/refresh` 成功则重放原请求，失败则推送 auth store 到 unauthenticated。

**Files:**
- Create: `apps/web/src/lib/csrf.ts`
- Create: `apps/web/src/lib/csrf.test.ts`
- Create: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/lib/api.test.ts`

- [ ] **Step 1: csrf 工具 + 测试**

Create `apps/web/src/lib/csrf.ts`:
```ts
const CSRF_COOKIE_NAME = 'tm_csrf_token'

/**
 * Read the tm_csrf_token cookie. Backend sets it non-httpOnly specifically so
 * JS can mirror it back via the X-CSRF-Token header (double-submit pattern).
 * Returns empty string when the cookie is not present.
 */
export function readCsrfToken(): string {
  const match = document.cookie
    .split('; ')
    .find((row) => row.startsWith(`${CSRF_COOKIE_NAME}=`))
  if (!match) return ''
  return decodeURIComponent(match.slice(CSRF_COOKIE_NAME.length + 1))
}
```

Create `apps/web/src/lib/csrf.test.ts`:
```ts
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { readCsrfToken } from './csrf'

describe('readCsrfToken', () => {
  beforeEach(() => {
    // Clear any existing cookies for a clean slate.
    document.cookie.split(';').forEach((c) => {
      const name = c.split('=')[0].trim()
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
    })
  })

  afterEach(() => {
    document.cookie = `tm_csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
  })

  it('returns empty string when cookie is absent', () => {
    expect(readCsrfToken()).toBe('')
  })

  it('returns cookie value when present', () => {
    document.cookie = 'tm_csrf_token=abc123; path=/'
    expect(readCsrfToken()).toBe('abc123')
  })

  it('returns decoded value when url-encoded', () => {
    document.cookie = 'tm_csrf_token=abc%2Fdef; path=/'
    expect(readCsrfToken()).toBe('abc/def')
  })

  it('does not confuse with other cookies sharing a prefix', () => {
    document.cookie = 'tm_csrf_token_other=wrong; path=/'
    document.cookie = 'tm_csrf_token=right; path=/'
    expect(readCsrfToken()).toBe('right')
  })
})
```

- [ ] **Step 2: 跑测试看通过**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/lib/csrf.test.ts
```
Expected: 4 PASS。

- [ ] **Step 3: 写 api 失败测试**

Create `apps/web/src/lib/api.test.ts`:
```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { AxiosHeaders } from 'axios'
import { api, __resetAuthInterceptorState } from './api'
import { useAuthStore } from '@/stores/auth-store'

describe('api axios instance', () => {
  beforeEach(() => {
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
    document.cookie = `tm_csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
  })

  it('base config has withCredentials and /api/v1 baseURL', () => {
    expect(api.defaults.withCredentials).toBe(true)
    expect(api.defaults.baseURL).toBe('/api/v1')
  })

  it('request interceptor injects X-CSRF-Token from cookie', async () => {
    document.cookie = 'tm_csrf_token=token-123; path=/'
    const interceptor = api.interceptors.request.handlers[0]
    const config = {
      headers: new AxiosHeaders(),
      method: 'post',
      url: '/anything',
    }
    const result = await interceptor.fulfilled(config)
    expect(result.headers.get('X-CSRF-Token')).toBe('token-123')
  })

  it('request interceptor does not attach header when cookie absent', async () => {
    const interceptor = api.interceptors.request.handlers[0]
    const config = {
      headers: new AxiosHeaders(),
      method: 'post',
      url: '/anything',
    }
    const result = await interceptor.fulfilled(config)
    expect(result.headers.get('X-CSRF-Token')).toBeFalsy()
  })

  it('401 response triggers single refresh attempt and replays original', async () => {
    const replaySpy = vi.fn().mockResolvedValue({ data: { ok: true }, status: 200 })
    const refreshSpy = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: {}, status: 200 } as never)

    const rejected = api.interceptors.response.handlers[0].rejected
    const originalConfig = {
      url: '/auth/me',
      method: 'get',
      headers: new AxiosHeaders(),
    }
    const requestFn = vi.spyOn(api, 'request').mockImplementationOnce(replaySpy)

    await rejected({
      config: originalConfig,
      response: { status: 401 },
      isAxiosError: true,
    }).catch(() => undefined)

    expect(refreshSpy).toHaveBeenCalledWith('/auth/refresh')
    expect(requestFn).toHaveBeenCalledTimes(1)
    refreshSpy.mockRestore()
    requestFn.mockRestore()
  })

  it('second 401 within one request chain rejects and marks unauthenticated', async () => {
    vi.spyOn(api, 'post').mockRejectedValueOnce({
      response: { status: 401 },
      isAxiosError: true,
    } as never)
    const rejected = api.interceptors.response.handlers[0].rejected

    const originalConfig = {
      url: '/auth/me',
      method: 'get',
      headers: new AxiosHeaders(),
    }

    await expect(
      rejected({
        config: originalConfig,
        response: { status: 401 },
        isAxiosError: true,
      }),
    ).rejects.toBeDefined()
    expect(useAuthStore.getState().auth.status).toBe('unauthenticated')
  })
})
```

- [ ] **Step 4: 跑测试看失败**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/lib/api.test.ts
```
Expected: "Cannot find module './api'" 或 undefined `api`。

- [ ] **Step 5: 写 api 实现**

Create `apps/web/src/lib/api.ts`:
```ts
import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth-store'
import { readCsrfToken } from './csrf'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export const api = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})

type RetryableConfig = InternalAxiosRequestConfig & { __retried?: boolean }

let refreshInFlight: Promise<void> | null = null

/**
 * Reset in-memory state shared across test runs. Do NOT call in prod.
 */
export function __resetAuthInterceptorState(): void {
  refreshInFlight = null
}

api.interceptors.request.use((config) => {
  if (config.method && config.method.toLowerCase() !== 'get') {
    const token = readCsrfToken()
    if (token) {
      config.headers.set('X-CSRF-Token', token)
    }
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetryableConfig | undefined
    const status = error.response?.status

    // Only intercept 401, only retry once, and never re-enter /auth/refresh itself.
    if (
      status === 401 &&
      original &&
      !original.__retried &&
      !original.url?.endsWith('/auth/refresh')
    ) {
      original.__retried = true

      try {
        if (!refreshInFlight) {
          refreshInFlight = api
            .post('/auth/refresh')
            .then(() => undefined)
            .finally(() => {
              refreshInFlight = null
            })
        }
        await refreshInFlight
        return api.request(original)
      } catch (refreshErr) {
        useAuthStore.getState().auth.markUnauthenticated()
        throw refreshErr
      }
    }

    if (status === 401) {
      useAuthStore.getState().auth.markUnauthenticated()
    }
    throw error
  },
)
```

- [ ] **Step 6: 跑测试看通过**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/lib/api.test.ts
```
Expected: 5 PASS。

- [ ] **Step 7: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/lib/csrf.ts apps/web/src/lib/csrf.test.ts apps/web/src/lib/api.ts apps/web/src/lib/api.test.ts
git commit -m "$(cat <<'EOF'
feat(web): axios api client with CSRF header + single 401 refresh retry

Adds src/lib/api.ts exporting a configured axios instance: withCredentials
for httpOnly cookies, X-CSRF-Token header injection from tm_csrf_token
cookie (double-submit), and a response interceptor that calls /auth/refresh
exactly once per original request on 401 before replaying it. Repeated
401s mark the auth store unauthenticated so routing can react.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: TanStack Query hooks 与共享 meQueryOptions

目标：三个 hook 分装 API 调用 + 一个共享 `meQueryOptions`（Task 6 的 `beforeLoad` 要用到）。

**Files:**
- Create: `apps/web/src/features/auth/hooks/me-query.ts`
- Create: `apps/web/src/features/auth/hooks/use-me.ts`
- Create: `apps/web/src/features/auth/hooks/use-login.ts`
- Create: `apps/web/src/features/auth/hooks/use-logout.ts`
- Create: `apps/web/src/features/auth/hooks/index.ts`（re-export）
- Create: `apps/web/src/features/auth/hooks/hooks.test.tsx`

- [ ] **Step 1: 写失败测试**

Create `apps/web/src/features/auth/hooks/hooks.test.tsx`:
```tsx
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactNode } from 'react'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { useMe } from './use-me'
import { useLogin } from './use-login'
import { useLogout } from './use-logout'

const user: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Root',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

function wrap(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

describe('auth hooks', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
    vi.restoreAllMocks()
  })

  it('useMe fetches and stores the user', async () => {
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { user } } as never)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    const { result } = renderHook(() => useMe(), { wrapper: wrap(client) })
    await waitFor(() => expect(result.current.data).toEqual(user))
    expect(useAuthStore.getState().auth.status).toBe('authenticated')
  })

  it('useLogin posts credentials and updates store', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { user } } as never)
    const client = new QueryClient()

    const { result } = renderHook(() => useLogin(), { wrapper: wrap(client) })
    await result.current.mutateAsync({ email: 'admin@example.com', password: 'pw' })
    expect(useAuthStore.getState().auth.user).toEqual(user)
  })

  it('useLogout posts and resets store', async () => {
    useAuthStore.getState().auth.setUser(user)
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: {} } as never)
    const client = new QueryClient()

    const { result } = renderHook(() => useLogout(), { wrapper: wrap(client) })
    await result.current.mutateAsync()
    expect(useAuthStore.getState().auth.status).toBe('unauthenticated')
  })
})
```

（该测试依赖 `@testing-library/react`，模板的 `package.json` 是否已有请先确认；如果没有：）

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web add -D @testing-library/react
```

- [ ] **Step 2: 跑测试看失败**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/features/auth/hooks/hooks.test.tsx
```
Expected: Cannot find module 类报错。

- [ ] **Step 3: 写 me-query.ts**

Create `apps/web/src/features/auth/hooks/me-query.ts`:
```ts
import { queryOptions } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

interface MeResponse {
  user: AuthUser
}

export const ME_QUERY_KEY = ['auth', 'me'] as const

export const meQueryOptions = queryOptions({
  queryKey: ME_QUERY_KEY,
  queryFn: async () => {
    const { data } = await api.get<MeResponse>('/auth/me')
    useAuthStore.getState().auth.setUser(data.user)
    return data.user
  },
  staleTime: 60 * 1000,
  retry: false,
})
```

- [ ] **Step 4: 写 use-me.ts**

Create `apps/web/src/features/auth/hooks/use-me.ts`:
```ts
import { useQuery } from '@tanstack/react-query'
import { meQueryOptions } from './me-query'

export function useMe() {
  return useQuery(meQueryOptions)
}
```

- [ ] **Step 5: 写 use-login.ts**

Create `apps/web/src/features/auth/hooks/use-login.ts`:
```ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { ME_QUERY_KEY } from './me-query'

export interface LoginInput {
  email: string
  password: string
}

interface LoginResponse {
  user: AuthUser
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: LoginInput) => {
      const { data } = await api.post<LoginResponse>('/auth/login', input)
      return data.user
    },
    onSuccess: (user) => {
      useAuthStore.getState().auth.setUser(user)
      qc.setQueryData(ME_QUERY_KEY, user)
    },
  })
}
```

- [ ] **Step 6: 写 use-logout.ts**

Create `apps/web/src/features/auth/hooks/use-logout.ts`:
```ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { ME_QUERY_KEY } from './me-query'

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.post('/auth/logout')
    },
    onSuccess: () => {
      useAuthStore.getState().auth.markUnauthenticated()
      qc.removeQueries({ queryKey: ME_QUERY_KEY })
    },
  })
}
```

- [ ] **Step 7: 写 index.ts**

Create `apps/web/src/features/auth/hooks/index.ts`:
```ts
export { useMe } from './use-me'
export { useLogin, type LoginInput } from './use-login'
export { useLogout } from './use-logout'
export { meQueryOptions, ME_QUERY_KEY } from './me-query'
```

- [ ] **Step 8: 跑测试看通过**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/features/auth/hooks/hooks.test.tsx
```
Expected: 3 PASS。

- [ ] **Step 9: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/auth/hooks/ apps/web/package.json apps/web/pnpm-lock.yaml
git commit -m "$(cat <<'EOF'
feat(web): auth hooks (useMe / useLogin / useLogout) and shared meQueryOptions

useLogin and useLogout mirror /auth/login and /auth/logout into the zustand
store and the react-query cache. meQueryOptions is the canonical fetcher
for the current user — shared by the useMe hook and the route guard's
beforeLoad prefetch so the two don't drift apart.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Sign-in 页面中文化并接入 useLogin

目标：实际登录跑通。去掉 `sleep(2000)` 假流程；用 `useLogin` mutation；删除 GitHub/Facebook 按钮；去掉 "Sign Up" 链接（MVP 不自助注册）；所有文案改中文。

**Files:**
- Rewrite: `apps/web/src/features/auth/sign-in/components/user-auth-form.tsx`
- Modify: `apps/web/src/features/auth/sign-in/index.tsx`

- [ ] **Step 1: 重写 user-auth-form.tsx**

Replace `apps/web/src/features/auth/sign-in/components/user-auth-form.tsx` 全文：
```tsx
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { Loader2, LogIn } from 'lucide-react'
import { toast } from 'sonner'
import { useLogin } from '@/features/auth/hooks'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'

const formSchema = z.object({
  email: z.email({
    error: (iss) => (iss.input === '' ? '请输入邮箱' : undefined),
  }),
  password: z.string().min(1, '请输入密码'),
})

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const navigate = useNavigate()
  const login = useLogin()

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { email: '', password: '' },
  })

  function onSubmit(data: z.infer<typeof formSchema>) {
    login.mutate(
      { email: data.email, password: data.password },
      {
        onSuccess: (user) => {
          toast.success(`欢迎回来，${user.name}`)
          navigate({ to: redirectTo || '/', replace: true })
        },
        onError: (err) => {
          if (err instanceof AxiosError) {
            const code = err.response?.data?.code
            if (code === 'ERR_INVALID_CREDENTIALS') {
              toast.error('邮箱或密码错误')
              return
            }
            if (code === 'ERR_USER_DISABLED') {
              toast.error('该账号已被停用，请联系管理员')
              return
            }
          }
          toast.error('登录失败，请稍后重试')
        },
      },
    )
  }

  const isLoading = login.isPending

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-3', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>邮箱</FormLabel>
              <FormControl>
                <Input placeholder='name@example.com' autoComplete='email' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>密码</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder='********'
                  autoComplete='current-password'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2' disabled={isLoading}>
          {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
          登录
        </Button>
      </form>
    </Form>
  )
}
```

- [ ] **Step 2: 改 sign-in/index.tsx 中文化 + 删 Sign Up 链接**

Replace `apps/web/src/features/auth/sign-in/index.tsx` 全文：
```tsx
import { useSearch } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  // strict:false lets this component render outside the `(auth)/sign-in` file
  // route (e.g. under the integration test's ad-hoc router) without TanStack
  // Router throwing "route id not found".
  const search = useSearch({ strict: false }) as { redirect?: string }

  return (
    <AuthLayout>
      <Card className='max-w-sm gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>登录</CardTitle>
          <CardDescription>
            输入账号邮箱与密码登录系统。如需开通账号，请联系系统管理员。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={search.redirect} />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
```

- [ ] **Step 3: 跑 typecheck + build**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
Expected: ts + build 全过。如果 Task 2 留下的 `@ts-expect-error` 占位仍存在，这里连带修掉（只要相关文件就是 user-auth-form.tsx）。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/auth/sign-in/
git commit -m "$(cat <<'EOF'
feat(web): wire sign-in form to useLogin mutation, localise to zh-CN

Replace the fake sleep(2000) mock with a real useLogin call: toast the
server-side error code (ERR_INVALID_CREDENTIALS / ERR_USER_DISABLED)
in Chinese, redirect to the originally requested page on success.
Remove the GitHub/Facebook OAuth buttons and the Sign Up link; MVP
does not support self-service registration.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: 路由守卫：_authenticated 的 beforeLoad 预取 me

目标：进入任何 `/_authenticated/*` 路由之前，预取 `['auth','me']`；拿不到（401 且 refresh 失败）则跳转 `/sign-in?redirect=<currentPath>`。

**Files:**
- Modify: `apps/web/src/main.tsx` (router 已经传 queryClient 到 context；确认类型)
- Modify: `apps/web/src/routes/_authenticated/route.tsx`

- [ ] **Step 1: 检查 router context**

Read `apps/web/src/main.tsx` 第 76-82 行左右 `createRouter({ context: { queryClient }, ... })`。应该已经有 `context: { queryClient }`。如果有则不动；如果没有，加上。

- [ ] **Step 2: 改 _authenticated/route.tsx**

Replace `apps/web/src/routes/_authenticated/route.tsx` 全文：
```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { meQueryOptions } from '@/features/auth/hooks'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.ensureQueryData(meQueryOptions)
    } catch {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: AuthenticatedLayout,
})
```

- [ ] **Step 3: 手动验证（dev server）**

```bash
cd /Users/adam/workspace/github/trademark-admin
# 后端必须跑起来
docker compose up -d postgres
cd apps/api
go build -o /tmp/tm-api ./cmd/server
DATABASE_URL=postgres://trademark:trademark@localhost:5432/trademark?sslmode=disable \
JWT_ACCESS_SECRET=dev-access \
JWT_REFRESH_SECRET=dev-refresh \
BOOTSTRAP_ADMIN_EMAIL=admin@example.com \
BOOTSTRAP_ADMIN_PASSWORD=change-me-on-first-login \
APP_ENV=development \
  /tmp/tm-api &
API_PID=$!
cd /Users/adam/workspace/github/trademark-admin

# 前端 dev
pnpm -C apps/web dev &
WEB_PID=$!
sleep 3

# 浏览器访问 http://localhost:5173
# 期望：自动跳到 /sign-in?redirect=...
# 输入 admin@example.com / change-me-on-first-login
# 期望：跳回 / 看到 Dashboard
# 期望：刷新页面依然保持登录（me 缓存 + cookie）

# 清理
kill $WEB_PID $API_PID
rm -f /tmp/tm-api
docker compose down -v
```

本 step 以手动 happy path 为准；记录观察结果但不必写自动化。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/routes/_authenticated/route.tsx apps/web/src/main.tsx
git commit -m "$(cat <<'EOF'
feat(web): route guard on _authenticated prefetches /auth/me via beforeLoad

TanStack Router's beforeLoad runs ensureQueryData(meQueryOptions) before
rendering any protected layout. A 401 (or refresh failure) throws a
redirect to /sign-in carrying the current href so we can bounce back
after login.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: 登出按钮 + 停用 sign-up / forgot-password / otp 路由

目标：1) profile 下拉的"退出登录"实际调 /auth/logout；2) 三个自助注册类路由替换为"请联系管理员"页面，防止用户闯进去看到 404 或模板样貌。

**Files:**
- Modify: `apps/web/src/components/layout/profile-dropdown.tsx`（或 user-menu 等现有 sign-out 位置）
- Rewrite: `apps/web/src/routes/(auth)/sign-up.tsx`
- Rewrite: `apps/web/src/routes/(auth)/forgot-password.tsx`
- Rewrite: `apps/web/src/routes/(auth)/otp.tsx`

- [ ] **Step 1: 找到现有 sign-out 位置**

```bash
cd /Users/adam/workspace/github/trademark-admin
grep -rn "Sign out\|signOut\|sign-out\|Log out" apps/web/src/components apps/web/src/features
```

记下哪个文件有登出按钮（大概率 `components/layout/profile-dropdown.tsx` 或 `components/profile-dropdown.tsx`）。

- [ ] **Step 2: 接 useLogout**

打开找到的文件，定位到 "Sign out" / "退出登录" 按钮。替换其 `onClick` 为：
```tsx
const logout = useLogout()
const navigate = useNavigate()
// ...
<DropdownMenuItem
  onClick={() =>
    logout.mutate(undefined, {
      onSettled: () => navigate({ to: '/sign-in', replace: true }),
    })
  }
>
  退出登录
</DropdownMenuItem>
```

需要相应 import：
```ts
import { useNavigate } from '@tanstack/react-router'
import { useLogout } from '@/features/auth/hooks'
```

同时把按钮文字（原模板多半是 "Sign out" 或 "Log out"）改为中文 "退出登录"；相邻的 "Profile / Account settings / Billing" 等菜单项中文化到位（如果这些后续 Plan 才实现，只改文字不改行为）。

- [ ] **Step 3: 停用 sign-up / forgot-password / otp 路由**

创建统一的提示组件 `apps/web/src/features/auth/contact-admin-notice.tsx`：
```tsx
import { Link } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { AuthLayout } from './auth-layout'

export function ContactAdminNotice({ title }: { title: string }) {
  return (
    <AuthLayout>
      <Card className='max-w-sm gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>{title}</CardTitle>
          <CardDescription>
            本系统不支持自助注册或找回密码。如需开通账号或重置密码，请联系系统管理员。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button asChild variant='outline' className='w-full'>
            <Link to='/sign-in'>返回登录</Link>
          </Button>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
```

然后替换三个 route 文件：

Replace `apps/web/src/routes/(auth)/sign-up.tsx`:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ContactAdminNotice } from '@/features/auth/contact-admin-notice'

export const Route = createFileRoute('/(auth)/sign-up')({
  component: () => <ContactAdminNotice title='无法自助注册' />,
})
```

Replace `apps/web/src/routes/(auth)/forgot-password.tsx`:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ContactAdminNotice } from '@/features/auth/contact-admin-notice'

export const Route = createFileRoute('/(auth)/forgot-password')({
  component: () => <ContactAdminNotice title='无法自助找回密码' />,
})
```

Replace `apps/web/src/routes/(auth)/otp.tsx`:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { ContactAdminNotice } from '@/features/auth/contact-admin-notice'

export const Route = createFileRoute('/(auth)/otp')({
  component: () => <ContactAdminNotice title='暂不支持验证码登录' />,
})
```

同时把 `apps/web/src/features/auth/sign-up/`、`apps/web/src/features/auth/forgot-password/`、`apps/web/src/features/auth/otp/` 三个目录下的旧实现文件：如果 route 已经不 import 它们，可以用 `git rm -r` 删除三个目录。

```bash
cd /Users/adam/workspace/github/trademark-admin
rm -rf apps/web/src/features/auth/sign-up apps/web/src/features/auth/forgot-password apps/web/src/features/auth/otp
```

- [ ] **Step 4: build 验证**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
Expected: 成功。如果还有对 `@/features/auth/{sign-up,forgot-password,otp}` 的 import 残留（比如 `sign-in-2.tsx` 里），把该 import 删除或改为指向同 sign-in 目录即可。

- [ ] **Step 5: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/auth/ apps/web/src/routes/\(auth\)/ apps/web/src/components/
git commit -m "$(cat <<'EOF'
feat(web): wire logout button + disable self-service signup / forgot-password

Logout dropdown now calls useLogout (POST /auth/logout) and bounces the
user to /sign-in. sign-up / forgot-password / otp routes render a 'contact
admin' notice in Chinese instead of the template fake forms, matching
MVP scope: administrators provision accounts and reset passwords.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: 浏览器端到端测试（MSW + Vitest browser）

目标：一个集成测试走通"未登录 → 访问 / → 跳 /sign-in → 输入凭证 → 跳 / → sidebar 显示用户名"的完整流程，用 MSW 拦截 `/api/v1/auth/*` 做 fixture，确保路由守卫 + 登录 + 状态回写都连通。

**Files:**
- Add dev-dep: `msw`
- Create: `apps/web/src/test-utils/msw/handlers.ts`
- Create: `apps/web/src/test-utils/msw/server.ts`
- Create: `apps/web/src/features/auth/sign-in/sign-in.integration.test.tsx`

- [ ] **Step 1: 安装 msw**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web add -D msw
```

- [ ] **Step 2: 写 MSW handlers**

Create `apps/web/src/test-utils/msw/handlers.ts`:
```ts
import { http, HttpResponse } from 'msw'
import type { AuthUser } from '@/stores/auth-store'

const adminUser: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Bootstrap Admin',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

let loggedIn = false

export const defaultHandlers = [
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = (await request.json()) as { email: string; password: string }
    if (body.email === 'admin@example.com' && body.password === 'change-me-on-first-login') {
      loggedIn = true
      return HttpResponse.json({ user: adminUser }, { status: 200 })
    }
    return HttpResponse.json(
      { code: 'ERR_INVALID_CREDENTIALS', message: 'email or password incorrect' },
      { status: 401 },
    )
  }),
  http.get('/api/v1/auth/me', () => {
    if (loggedIn) return HttpResponse.json({ user: adminUser })
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'authentication required' },
      { status: 401 },
    )
  }),
  http.post('/api/v1/auth/refresh', () => {
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'no refresh token' },
      { status: 401 },
    )
  }),
  http.post('/api/v1/auth/logout', () => {
    loggedIn = false
    return new HttpResponse(null, { status: 204 })
  }),
]

export function resetMswState() {
  loggedIn = false
}
```

- [ ] **Step 3: 写 server.ts (browser worker)**

Create `apps/web/src/test-utils/msw/server.ts`:
```ts
import { setupWorker } from 'msw/browser'
import { defaultHandlers } from './handlers'

export const worker = setupWorker(...defaultHandlers)
```

- [ ] **Step 4: 写集成测试**

Create `apps/web/src/features/auth/sign-in/sign-in.integration.test.tsx`:
```tsx
import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
  createRootRoute,
  createRoute,
  Outlet,
  redirect,
} from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { worker } from '@/test-utils/msw/server'
import { resetMswState } from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { SignIn } from '.'
import { meQueryOptions } from '@/features/auth/hooks'

function buildRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  const rootRoute = createRootRoute({ component: () => <Outlet /> })

  const signInRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/sign-in',
    validateSearch: (search: Record<string, unknown>) => ({
      redirect: (search.redirect as string) ?? '',
    }),
    component: SignIn,
  })

  const homeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    beforeLoad: async ({ location }) => {
      try {
        await queryClient.ensureQueryData(meQueryOptions)
      } catch {
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    },
    component: () => {
      const user = useAuthStore((s) => s.auth.user)
      return <div>Welcome {user?.name}</div>
    },
  })

  const router = createRouter({
    routeTree: rootRoute.addChildren([signInRoute, homeRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
    context: { queryClient },
  })

  return { router, queryClient }
}

describe('sign-in integration', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'error' })
  })

  beforeEach(() => {
    resetMswState()
    useAuthStore.getState().auth.reset()
  })

  afterAll(() => {
    worker.stop()
  })

  it('unauthenticated root → sign-in → successful login → welcome', async () => {
    const { router, queryClient } = buildRouter()
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // beforeLoad on / throws into /sign-in
    await waitFor(() => expect(screen.getByText('登录')).toBeInTheDocument())

    await userEvent.type(
      screen.getByLabelText('邮箱'),
      'admin@example.com',
    )
    await userEvent.type(
      screen.getByLabelText('密码'),
      'change-me-on-first-login',
    )
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    await waitFor(() =>
      expect(screen.getByText('Welcome Bootstrap Admin')).toBeInTheDocument(),
    )
  })

  it('wrong credentials show error toast and stay on sign-in', async () => {
    const { router, queryClient } = buildRouter()
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('登录')).toBeInTheDocument())
    await userEvent.type(screen.getByLabelText('邮箱'), 'admin@example.com')
    await userEvent.type(screen.getByLabelText('密码'), 'wrong-password')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    await waitFor(() =>
      expect(screen.getByText('邮箱或密码错误')).toBeInTheDocument(),
    )
    // Still on sign-in: Welcome text from home route must not have rendered.
    expect(screen.queryByText(/Welcome/)).not.toBeInTheDocument()
  })
})
```

Install `@testing-library/user-event` as devDep if missing:
```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web add -D @testing-library/user-event
```

- [ ] **Step 5: 跑测试看通过**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/features/auth/sign-in/sign-in.integration.test.tsx
```
Expected: 2 PASS。

如果因 MSW service worker 注册失败（vitest browser 模式第一次要求 `mockServiceWorker.js`），按提示跑 `pnpm -C apps/web exec msw init public --save`，会生成 `apps/web/public/mockServiceWorker.js`，把它也加入提交。

- [ ] **Step 6: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/test-utils/msw/ apps/web/src/features/auth/sign-in/sign-in.integration.test.tsx apps/web/package.json apps/web/pnpm-lock.yaml apps/web/public/mockServiceWorker.js
git commit -m "$(cat <<'EOF'
test(web): integration test covering guard → sign-in → home round trip

Uses MSW inside vitest browser mode to simulate /api/v1/auth/{login,me,
logout,refresh}. Validates that beforeLoad bounces unauthenticated users
to /sign-in, that the form maps ERR_INVALID_CREDENTIALS to the Chinese
error toast, and that a successful login hydrates the user into the
route-guarded home page.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Plan 3 结束状态清单（Definition of Done）

1. ✅ `pnpm -C apps/web build` 成功，无 `@clerk/react` 引用
2. ✅ `pnpm -C apps/web test --run` 全绿（auth-store + csrf + api + hooks + sign-in integration）
3. ✅ `pnpm -C apps/web lint` 无错误
4. ✅ `apps/web/src/routes/clerk/` 已删除
5. ✅ `apps/web/src/lib/api.ts` 作为唯一 axios 出口，所有新业务调用都用它
6. ✅ 未登录访问 `/` 自动跳 `/sign-in?redirect=/`
7. ✅ 正确凭证登录后跳转到原 redirect 页面
8. ✅ 错误凭证显示 "邮箱或密码错误" toast
9. ✅ 停用账号显示 "该账号已被停用，请联系管理员" toast
10. ✅ 登录后刷新页面仍保持登录（通过 cookie）
11. ✅ 登出按钮调 /auth/logout 清 cookie + 跳 /sign-in
12. ✅ /sign-up /forgot-password /otp 显示"联系管理员"中文提示

## 下一步

Plan 3 完成后进入 **Plan 4（字典 + 客户档案）**：Nice 分类 45 项 seed、约 130 国家字典 seed（后端 migration + seed 数据）；前端 `/customers` 和 `/catalog/countries` 路由 + TanStack Table 列表 + 创建/编辑表单。
