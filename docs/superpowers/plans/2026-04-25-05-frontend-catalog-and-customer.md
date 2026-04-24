# Plan 5: 前端字典 + 客户档案视图 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Plan 4 交付的后端 `/api/v1/catalog/*` 与 `/api/v1/customers` 端点接到前端页面：客户档案列表 + 详情 + 创建/编辑表单、国家字典（admin 可编辑）、尼斯分类（只读）、sidebar 导航按角色可见。并用一条 MSW 集成测试验证"登录 → 建客户 → 列表" 闭环。

**Architecture:** 每个业务域在 `features/` 下单独一个目录，内部按 `api.ts / schema.ts / hooks.ts / components/` 分层。列表页用 TanStack Table；客户列表走服务端分页（q / page / page_size 走 URL search params 并塞到 axios），国家/尼斯分类因为量小（61 + 45）用客户端分页即可。表单统一用 react-hook-form + zod + shadcn `Form`。角色门卫走 TanStack Router 的 `beforeLoad`：admin-only 路由在 beforeLoad 里读 `meQueryOptions` 缓存的 user，若 role !== 'admin' 抛 redirect 到 `/403`。sidebar 的 navGroups 按角色动态裁剪。

**Tech Stack:** React 19 + Vite + TanStack Router + TanStack Query (`/api/v1/*` via `apps/web/src/lib/api`) + @tanstack/react-table v8 + react-hook-form + zod + Sonner（toast）+ shadcn/ui + MSW（browser worker，已由 Plan 3 引入）+ Vitest browser mode（playwright chromium，已配置）。

---

## File Structure

### Create

**共享类型**
- `apps/web/src/features/catalog/types.ts` — `Country`, `NiceCategory`, `UpdateCountryRequest` DTOs 的 TS 复刻
- `apps/web/src/features/customers/types.ts` — `Customer`, `CreateCustomerRequest`, `UpdateCustomerRequest`, `CustomerListResponse`

**API hooks（每个业务域一个文件）**
- `apps/web/src/features/catalog/hooks/use-countries.ts`
- `apps/web/src/features/catalog/hooks/use-nice-categories.ts`
- `apps/web/src/features/catalog/hooks/use-update-country.ts`
- `apps/web/src/features/catalog/hooks/index.ts` — 桶式 re-export
- `apps/web/src/features/customers/hooks/use-customers.ts`（列表 + 详情）
- `apps/web/src/features/customers/hooks/use-customer-mutations.ts`（create / update）
- `apps/web/src/features/customers/hooks/index.ts`

**Sidebar / 角色门卫**
- `apps/web/src/components/layout/data/sidebar-data.ts` — 重写为按当前用户角色组装 navGroups（见 Task 4）
- `apps/web/src/components/layout/app-sidebar.tsx` — 改为读 useMe 来 derive 用户信息

**客户页面**
- `apps/web/src/routes/_authenticated/customers/index.tsx`
- `apps/web/src/routes/_authenticated/customers/$id.tsx`
- `apps/web/src/features/customers/index.tsx` — 列表页
- `apps/web/src/features/customers/detail.tsx` — 详情页
- `apps/web/src/features/customers/components/customers-table.tsx`
- `apps/web/src/features/customers/components/customers-columns.tsx`
- `apps/web/src/features/customers/components/customer-form-dialog.tsx` — 创建/编辑复用

**字典页面**
- `apps/web/src/routes/_authenticated/catalog/countries.tsx`
- `apps/web/src/routes/_authenticated/catalog/nice-categories.tsx`
- `apps/web/src/features/catalog/countries.tsx`
- `apps/web/src/features/catalog/nice-categories.tsx`
- `apps/web/src/features/catalog/components/countries-table.tsx`
- `apps/web/src/features/catalog/components/country-edit-drawer.tsx`
- `apps/web/src/features/catalog/components/nice-categories-table.tsx`

**测试**
- `apps/web/src/test-utils/msw/handlers.ts` — 扩展以覆盖 catalog / customers（见 Task 10）
- `apps/web/src/features/customers/customers.integration.test.tsx` — 新增集成测试

### Modify

- `apps/web/src/components/layout/data/sidebar-data.ts`
- `apps/web/src/components/layout/app-sidebar.tsx`

---

## Task 1: 类型定义（catalog + customer）

**Files:**
- Create: `apps/web/src/features/catalog/types.ts`
- Create: `apps/web/src/features/customers/types.ts`

- [ ] **Step 1: catalog 类型**

Create `apps/web/src/features/catalog/types.ts`:
```ts
export interface Country {
  id: string
  code: string
  name_zh: string
  name_en: string
  is_madrid_member: boolean
  default_acceptance_days?: number | null
  default_registration_months?: number | null
  requires_notarization: boolean
  notes_zh?: string | null
  notes_en?: string | null
  sort_order: number
  enabled: boolean
}

export interface NiceCategory {
  code: number
  name_zh: string
  name_en: string
  description_zh?: string | null
  description_en?: string | null
}

export interface UpdateCountryRequest {
  name_zh?: string
  name_en?: string
  is_madrid_member?: boolean
  default_acceptance_days?: number | null
  default_registration_months?: number | null
  requires_notarization?: boolean
  notes_zh?: string | null
  notes_en?: string | null
  sort_order?: number
  enabled?: boolean
}

export interface ListEnvelope<T> {
  items: T[]
}
```

- [ ] **Step 2: customer 类型**

Create `apps/web/src/features/customers/types.ts`:
```ts
export interface Customer {
  id: string
  name: string
  industry?: string | null
  is_returning: boolean
  price_sensitive: boolean
  contact_name?: string | null
  contact_phone?: string | null
  contact_email?: string | null
  notes?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateCustomerRequest {
  name: string
  industry?: string | null
  is_returning?: boolean
  price_sensitive?: boolean
  contact_name?: string | null
  contact_phone?: string | null
  contact_email?: string | null
  notes?: string | null
}

export type UpdateCustomerRequest = Partial<CreateCustomerRequest>

export interface CustomerListResponse {
  items: Customer[]
  page: number
  page_size: number
  total: number
}

export interface CustomerListQuery {
  q?: string
  page?: number
  page_size?: number
}
```

- [ ] **Step 3: build + typecheck**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
Expected: succeed (no TypeScript errors).

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/catalog/types.ts apps/web/src/features/customers/types.ts
git commit -m "$(cat <<'EOF'
feat(web): TS types for catalog + customer domain

Country / NiceCategory / UpdateCountryRequest plus Customer / CRUD
request envelopes + CustomerListResponse with pagination metadata.
Faithful mirror of the Go DTOs under apps/api/internal/{catalog,customer}
so subsequent hooks can import these directly.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Catalog API hooks

**Files:**
- Create: `apps/web/src/features/catalog/hooks/use-countries.ts`
- Create: `apps/web/src/features/catalog/hooks/use-nice-categories.ts`
- Create: `apps/web/src/features/catalog/hooks/use-update-country.ts`
- Create: `apps/web/src/features/catalog/hooks/index.ts`

- [ ] **Step 1: useCountries**

Create `apps/web/src/features/catalog/hooks/use-countries.ts`:
```ts
import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { Country, ListEnvelope } from '../types'

export const COUNTRIES_QUERY_KEY = ['catalog', 'countries'] as const

export const countriesQueryOptions = (includeDisabled = false) =>
  queryOptions({
    queryKey: [...COUNTRIES_QUERY_KEY, { includeDisabled }] as const,
    queryFn: async (): Promise<Country[]> => {
      const res = await api.get<ListEnvelope<Country>>('/catalog/countries', {
        params: includeDisabled ? { include_disabled: true } : undefined,
      })
      return res.data.items
    },
    staleTime: 5 * 60 * 1000, // 5 min
  })

export function useCountries(includeDisabled = false) {
  return useQuery(countriesQueryOptions(includeDisabled))
}
```

- [ ] **Step 2: useNiceCategories**

Create `apps/web/src/features/catalog/hooks/use-nice-categories.ts`:
```ts
import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { NiceCategory, ListEnvelope } from '../types'

export const NICE_CATEGORIES_QUERY_KEY = ['catalog', 'nice-categories'] as const

export const niceCategoriesQueryOptions = queryOptions({
  queryKey: NICE_CATEGORIES_QUERY_KEY,
  queryFn: async (): Promise<NiceCategory[]> => {
    const res = await api.get<ListEnvelope<NiceCategory>>('/catalog/nice-categories')
    return res.data.items
  },
  staleTime: 60 * 60 * 1000, // 1 hour — basically static
})

export function useNiceCategories() {
  return useQuery(niceCategoriesQueryOptions)
}
```

- [ ] **Step 3: useUpdateCountry**

Create `apps/web/src/features/catalog/hooks/use-update-country.ts`:
```ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type { Country, UpdateCountryRequest } from '../types'
import { COUNTRIES_QUERY_KEY } from './use-countries'

export function useUpdateCountry() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; body: UpdateCountryRequest }): Promise<Country> => {
      const res = await api.patch<Country>(`/catalog/countries/${args.id}`, args.body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: COUNTRIES_QUERY_KEY })
      toast.success('国家信息已更新')
    },
    onError: (err) => {
      if (err instanceof AxiosError && err.response?.status === 403) {
        toast.error('没有权限修改字典')
        return
      }
      toast.error('更新失败，请稍后重试')
    },
  })
}
```

- [ ] **Step 4: index.ts 桶式**

Create `apps/web/src/features/catalog/hooks/index.ts`:
```ts
export * from './use-countries'
export * from './use-nice-categories'
export * from './use-update-country'
```

- [ ] **Step 5: build 验证**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
Expected: succeed.

- [ ] **Step 6: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/catalog/hooks/
git commit -m "$(cat <<'EOF'
feat(web): TanStack Query hooks for catalog (countries + nice categories)

countriesQueryOptions accepts includeDisabled flag and caches for 5 min.
niceCategoriesQueryOptions is essentially static (1h cache). useUpdateCountry
PATCHes /catalog/countries/:id and maps the 403 branch to a dedicated
Chinese toast before invalidating the countries cache.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Customer API hooks

**Files:**
- Create: `apps/web/src/features/customers/hooks/use-customers.ts`
- Create: `apps/web/src/features/customers/hooks/use-customer-mutations.ts`
- Create: `apps/web/src/features/customers/hooks/index.ts`

- [ ] **Step 1: list + detail query**

Create `apps/web/src/features/customers/hooks/use-customers.ts`:
```ts
import { queryOptions, keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  Customer,
  CustomerListQuery,
  CustomerListResponse,
} from '../types'

export const CUSTOMERS_QUERY_KEY = ['customers'] as const

export const customersListQueryOptions = (query: CustomerListQuery) =>
  queryOptions({
    queryKey: [...CUSTOMERS_QUERY_KEY, 'list', query] as const,
    queryFn: async (): Promise<CustomerListResponse> => {
      const res = await api.get<CustomerListResponse>('/customers', {
        params: {
          q: query.q || undefined,
          page: query.page ?? 1,
          page_size: query.page_size ?? 20,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const customerDetailQueryOptions = (id: string) =>
  queryOptions({
    queryKey: [...CUSTOMERS_QUERY_KEY, 'detail', id] as const,
    queryFn: async (): Promise<Customer> => {
      const res = await api.get<Customer>(`/customers/${id}`)
      return res.data
    },
  })

export function useCustomersList(query: CustomerListQuery) {
  return useQuery(customersListQueryOptions(query))
}

export function useCustomer(id: string) {
  return useQuery(customerDetailQueryOptions(id))
}
```

- [ ] **Step 2: create + update**

Create `apps/web/src/features/customers/hooks/use-customer-mutations.ts`:
```ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  Customer,
  CreateCustomerRequest,
  UpdateCustomerRequest,
} from '../types'
import { CUSTOMERS_QUERY_KEY } from './use-customers'

function mapCustomerError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_DUPLICATE_NAME') return '已存在同名客户'
    if (err.response?.status === 403) return '没有权限操作客户'
  }
  return '请求失败，请稍后重试'
}

export function useCreateCustomer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateCustomerRequest): Promise<Customer> => {
      const res = await api.post<Customer>('/customers', body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: CUSTOMERS_QUERY_KEY })
      toast.success('客户已创建')
    },
    onError: (err) => {
      toast.error(mapCustomerError(err))
    },
  })
}

export function useUpdateCustomer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      body: UpdateCustomerRequest
    }): Promise<Customer> => {
      const res = await api.patch<Customer>(`/customers/${args.id}`, args.body)
      return res.data
    },
    onSuccess: (data) => {
      void qc.invalidateQueries({ queryKey: CUSTOMERS_QUERY_KEY })
      qc.setQueryData(
        [...CUSTOMERS_QUERY_KEY, 'detail', data.id] as const,
        data
      )
      toast.success('客户已更新')
    },
    onError: (err) => {
      toast.error(mapCustomerError(err))
    },
  })
}
```

- [ ] **Step 3: index.ts**

Create `apps/web/src/features/customers/hooks/index.ts`:
```ts
export * from './use-customers'
export * from './use-customer-mutations'
```

- [ ] **Step 4: build + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
git add apps/web/src/features/customers/hooks/
git commit -m "$(cat <<'EOF'
feat(web): TanStack Query hooks for customers list/detail + create/update

List query accepts q / page / page_size and uses keepPreviousData so
pagination flicker-free. create/update mutations share error-code
mapping (ERR_DUPLICATE_NAME -> "已存在同名客户", 403 -> "没有权限操作客户").
Update also primes the detail cache via setQueryData so navigating to
the detail page after edit is instant.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Sidebar 按角色动态组装 + 中文化

**Files:**
- Modify: `apps/web/src/components/layout/data/sidebar-data.ts`
- Modify: `apps/web/src/components/layout/app-sidebar.tsx`

目标：
1. 砍掉模板残留的 Apps / Chats / Tasks / Clerk / Auth / Errors 等演示入口，保留 Dashboard + Settings + Help
2. 添加 "客户" (/customers) 对所有角色可见
3. 添加 "字典" 折叠组（countries + nice-categories），仅 admin 可见
4. "用户管理" (/admin/users) 仅 admin 可见（留 placeholder URL；后端在 Plan 2 已有但前端页面留到未来 Plan）— 本 Plan 先不做用户管理页，故不加链接
5. 用户信息 + avatar 来自 useMe 返回的 user（而非硬编码 satnaing）

- [ ] **Step 1: 重写 sidebar-data.ts 为函数**

Replace `apps/web/src/components/layout/data/sidebar-data.ts`:
```ts
import {
  BookOpen,
  Building2,
  Command,
  HelpCircle,
  LayoutDashboard,
  Settings,
  Users,
} from 'lucide-react'
import type { AuthUser } from '@/stores/auth-store'
import { type NavGroup, type SidebarData } from '../types'

function navGroupsFor(role: AuthUser['role']): NavGroup[] {
  const base: NavGroup[] = [
    {
      title: '主导航',
      items: [
        {
          title: '仪表盘',
          url: '/',
          icon: LayoutDashboard,
        },
        {
          title: '客户',
          url: '/customers',
          icon: Building2,
        },
      ],
    },
  ]

  if (role === 'admin') {
    base.push({
      title: '字典',
      items: [
        {
          title: '国家',
          url: '/catalog/countries',
          icon: BookOpen,
        },
        {
          title: '尼斯分类',
          url: '/catalog/nice-categories',
          icon: BookOpen,
        },
      ],
    })
  }

  base.push({
    title: '系统',
    items: [
      {
        title: '个人设置',
        url: '/settings',
        icon: Settings,
      },
      {
        title: '帮助',
        url: '/help-center',
        icon: HelpCircle,
      },
    ],
  })

  return base
}

export function buildSidebarData(user: AuthUser | null): SidebarData {
  return {
    user: user
      ? { name: user.name, email: user.email, avatar: '/avatars/01.png' }
      : { name: '—', email: '—', avatar: '/avatars/01.png' },
    teams: [
      {
        name: '商标报价平台',
        logo: Command,
        plan: '国际业务',
      },
    ],
    navGroups: user ? navGroupsFor(user.role) : [],
  }
}
```

注意：移除了 `satnaing` / `Shadcn Admin` / `Acme` / Clerk 等 demo 数据。如果其他地方还 `import { sidebarData }` 了旧的常量，会 break — Step 2 会把 app-sidebar 切换为 `buildSidebarData(user)`，其余使用者（大概率没有）同步修。先 grep 检查：

```bash
cd /Users/adam/workspace/github/trademark-admin
grep -rn "sidebarData" apps/web/src | grep -v "node_modules"
```
预期：只有 `apps/web/src/components/layout/app-sidebar.tsx` 一处引用。

- [ ] **Step 2: 改 app-sidebar.tsx 读 useMe**

Replace `apps/web/src/components/layout/app-sidebar.tsx`:
```tsx
import { useLayout } from '@/context/layout-provider'
import { useMe } from '@/features/auth/hooks'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
} from '@/components/ui/sidebar'
import { buildSidebarData } from './data/sidebar-data'
import { NavGroup } from './nav-group'
import { NavUser } from './nav-user'
import { TeamSwitcher } from './team-switcher'

export function AppSidebar() {
  const { collapsible, variant } = useLayout()
  const me = useMe()
  const sidebarData = buildSidebarData(me.data ?? null)

  return (
    <Sidebar collapsible={collapsible} variant={variant}>
      <SidebarHeader>
        <TeamSwitcher teams={sidebarData.teams} />
      </SidebarHeader>
      <SidebarContent>
        {sidebarData.navGroups.map((props) => (
          <NavGroup key={props.title} {...props} />
        ))}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={sidebarData.user} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
```

因为 `_authenticated/route.tsx` 的 beforeLoad 已经保证 me 缓存存在，这里 `useMe().data` 在渲染时应该一定有值（首次 render 不会命中 loading，因为数据在 router 的 beforeLoad 里已经 prefetch）。即便没有，我们给了空 user 的回退（teams 仍会显示，但 navGroups 为空）。

- [ ] **Step 3: build + 手动浏览器验证**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
手动验证留到 Task 8/9 再做，一起看整体 sidebar 样子。

- [ ] **Step 4: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/components/layout/data/sidebar-data.ts apps/web/src/components/layout/app-sidebar.tsx
git commit -m "$(cat <<'EOF'
feat(web): sidebar built from current user role + Chinese menu items

sidebar-data exports a buildSidebarData(user) builder: Dashboard +
Customers for everyone, Catalog (countries, nice-categories) only for
admin, Settings + Help tail. app-sidebar calls useMe() and hydrates
teams/user/navGroups from the resulting profile. Template demo clutter
(Apps / Chats / Tasks / Clerk / Pages / Errors) is dropped now that we
have a real navigation surface.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 客户列表页

**Files:**
- Create: `apps/web/src/routes/_authenticated/customers/index.tsx`
- Create: `apps/web/src/features/customers/index.tsx`
- Create: `apps/web/src/features/customers/components/customers-table.tsx`
- Create: `apps/web/src/features/customers/components/customers-columns.tsx`

- [ ] **Step 1: 路由 + search schema**

Create `apps/web/src/routes/_authenticated/customers/index.tsx`:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Customers } from '@/features/customers'
import { customersListQueryOptions } from '@/features/customers/hooks'

const searchSchema = z.object({
  q: z.string().optional().catch(''),
  page: z.number().int().min(1).optional().catch(1),
  page_size: z.number().int().min(1).max(100).optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/customers/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, deps }) =>
    context.queryClient.ensureQueryData(
      customersListQueryOptions({
        q: deps.search.q,
        page: deps.search.page,
        page_size: deps.search.page_size,
      })
    ),
  component: Customers,
})
```

- [ ] **Step 2: 列定义**

Create `apps/web/src/features/customers/components/customers-columns.tsx`:
```tsx
import { Link } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import type { Customer } from '../types'

export const customersColumns: ColumnDef<Customer>[] = [
  {
    accessorKey: 'name',
    header: '客户名称',
    cell: ({ row }) => (
      <Link
        to='/customers/$id'
        params={{ id: row.original.id }}
        className='font-medium text-primary underline-offset-4 hover:underline'
      >
        {row.original.name}
      </Link>
    ),
  },
  {
    accessorKey: 'industry',
    header: '行业',
    cell: ({ getValue }) => (getValue<string | null>() ?? '—'),
  },
  {
    accessorKey: 'is_returning',
    header: '回头客户',
    cell: ({ getValue }) =>
      getValue<boolean>() ? <Badge>回头</Badge> : <span className='text-muted-foreground'>—</span>,
  },
  {
    accessorKey: 'price_sensitive',
    header: '价格敏感',
    cell: ({ getValue }) =>
      getValue<boolean>() ? <Badge variant='secondary'>敏感</Badge> : <span className='text-muted-foreground'>—</span>,
  },
  {
    accessorKey: 'contact_name',
    header: '联系人',
    cell: ({ getValue }) => getValue<string | null>() ?? '—',
  },
  {
    accessorKey: 'contact_phone',
    header: '电话',
    cell: ({ getValue }) => getValue<string | null>() ?? '—',
  },
]
```

- [ ] **Step 3: 表格组件（服务端分页）**

Create `apps/web/src/features/customers/components/customers-table.tsx`:
```tsx
import { useState } from 'react'
import { getCoreRowModel, flexRender, useReactTable } from '@tanstack/react-table'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Customer } from '../types'
import { customersColumns } from './customers-columns'

interface Props {
  data: Customer[]
  total: number
  page: number
  pageSize: number
  queryText: string
  onQueryChange: (q: string) => void
  onPageChange: (page: number) => void
}

export function CustomersTable({
  data,
  total,
  page,
  pageSize,
  queryText,
  onQueryChange,
  onPageChange,
}: Props) {
  const [draft, setDraft] = useState(queryText)
  const table = useReactTable({
    data,
    columns: customersColumns,
    getCoreRowModel: getCoreRowModel(),
  })

  const pageCount = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className='flex flex-1 flex-col gap-4'>
      <form
        className='flex items-center gap-2'
        onSubmit={(e) => {
          e.preventDefault()
          onQueryChange(draft.trim())
        }}
      >
        <Input
          placeholder='按姓名或行业搜索'
          className='max-w-xs'
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <Button type='submit' variant='secondary'>
          搜索
        </Button>
        {queryText && (
          <Button
            type='button'
            variant='ghost'
            onClick={() => {
              setDraft('')
              onQueryChange('')
            }}
          >
            清除
          </Button>
        )}
      </form>
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={customersColumns.length} className='h-24 text-center'>
                  暂无客户
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className='flex items-center justify-between'>
        <span className='text-sm text-muted-foreground'>
          共 {total} 条，第 {page} / {pageCount} 页
        </span>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
          >
            上一页
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={page >= pageCount}
            onClick={() => onPageChange(page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: 页面组件**

Create `apps/web/src/features/customers/index.tsx`:
```tsx
import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useCustomersList } from './hooks'
import { CustomersTable } from './components/customers-table'
import { CustomerFormDialog } from './components/customer-form-dialog'

const route = getRouteApi('/_authenticated/customers/')

export function Customers() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const query = {
    q: search.q ?? '',
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }
  const { data, isLoading } = useCustomersList(query)
  const [createOpen, setCreateOpen] = useState(false)

  const setSearch = (patch: Partial<typeof search>) =>
    navigate({ search: (old) => ({ ...old, ...patch }), replace: false })

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>客户档案</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-1 flex-col gap-4'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>客户档案</h2>
            <p className='text-muted-foreground'>
              按角色可见：业务员只看自建，国际部商务与管理员看全部。
            </p>
          </div>
          <Button onClick={() => setCreateOpen(true)}>新建客户</Button>
        </div>
        <CustomersTable
          data={data?.items ?? []}
          total={data?.total ?? 0}
          page={query.page}
          pageSize={query.page_size}
          queryText={query.q}
          onQueryChange={(q) => setSearch({ q: q || undefined, page: 1 })}
          onPageChange={(page) => setSearch({ page })}
        />
        {isLoading && <p className='text-sm text-muted-foreground'>正在加载…</p>}
      </Main>
      <CustomerFormDialog mode='create' open={createOpen} onOpenChange={setCreateOpen} />
    </>
  )
}
```

- [ ] **Step 5: build 验证（此时 customer-form-dialog 还不存在，build 会 fail，留到 Task 6 补齐；先跳过 build，只编译其他文件）**

```bash
cd /Users/adam/workspace/github/trademark-admin
# 跳过 build —— 下一 task 会补齐缺的组件
```

- [ ] **Step 6: 提交（与 Task 6 紧邻，必要时可合并成一个 commit）**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/routes/_authenticated/customers/ apps/web/src/features/customers/index.tsx apps/web/src/features/customers/components/customers-table.tsx apps/web/src/features/customers/components/customers-columns.tsx
git commit -m "$(cat <<'EOF'
feat(web): customers list page with server-side pagination + search

Route /_authenticated/customers/ reads q/page/page_size from URL search
params, prefetches list via router loader, renders a TanStack Table with
Chinese column headers. Name cell is a Link to the detail route. Search
form stages local draft then applies on submit to avoid one-roundtrip-
per-keystroke. Pagination uses next/prev buttons; total + page_count
derived from the API envelope.

NOTE: CustomerFormDialog is imported but added in the next task —
build intentionally left red across the commit seam; Task 6 closes it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 客户创建/编辑表单

**Files:**
- Create: `apps/web/src/features/customers/components/customer-form-dialog.tsx`

- [ ] **Step 1: 表单 + 校验**

Create `apps/web/src/features/customers/components/customer-form-dialog.tsx`:
```tsx
import { useEffect } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import type { Customer } from '../types'
import { useCreateCustomer, useUpdateCustomer } from '../hooks'

const schema = z.object({
  name: z.string().min(1, '客户名称不能为空').max(200, '客户名称过长'),
  industry: z.string().max(200).optional().or(z.literal('')),
  is_returning: z.boolean(),
  price_sensitive: z.boolean(),
  contact_name: z.string().max(100).optional().or(z.literal('')),
  contact_phone: z.string().max(50).optional().or(z.literal('')),
  contact_email: z.string().email('邮箱格式不正确').max(200).optional().or(z.literal('')),
  notes: z.string().max(2000).optional().or(z.literal('')),
})

type FormValues = z.infer<typeof schema>

interface Props {
  mode: 'create' | 'edit'
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: Customer
}

const emptyValues: FormValues = {
  name: '',
  industry: '',
  is_returning: false,
  price_sensitive: false,
  contact_name: '',
  contact_phone: '',
  contact_email: '',
  notes: '',
}

export function CustomerFormDialog({ mode, open, onOpenChange, initial }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: emptyValues,
  })
  const createMut = useCreateCustomer()
  const updateMut = useUpdateCustomer()

  useEffect(() => {
    if (!open) return
    if (mode === 'edit' && initial) {
      form.reset({
        name: initial.name,
        industry: initial.industry ?? '',
        is_returning: initial.is_returning,
        price_sensitive: initial.price_sensitive,
        contact_name: initial.contact_name ?? '',
        contact_phone: initial.contact_phone ?? '',
        contact_email: initial.contact_email ?? '',
        notes: initial.notes ?? '',
      })
    } else {
      form.reset(emptyValues)
    }
  }, [open, mode, initial, form])

  const onSubmit = form.handleSubmit(async (values) => {
    // Normalise empty strings to null so backend stores NULL, not "".
    const payload = {
      name: values.name,
      industry: values.industry || null,
      is_returning: values.is_returning,
      price_sensitive: values.price_sensitive,
      contact_name: values.contact_name || null,
      contact_phone: values.contact_phone || null,
      contact_email: values.contact_email || null,
      notes: values.notes || null,
    }

    try {
      if (mode === 'edit' && initial) {
        await updateMut.mutateAsync({ id: initial.id, body: payload })
      } else {
        await createMut.mutateAsync(payload)
      }
      onOpenChange(false)
    } catch {
      // Toast is already shown by the mutation's onError; keep dialog open.
    }
  })

  const busy = createMut.isPending || updateMut.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{mode === 'edit' ? '编辑客户' : '新建客户'}</DialogTitle>
          <DialogDescription>
            {mode === 'edit' ? '修改客户信息后点击保存。' : '填写客户基本信息并保存。'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='grid grid-cols-2 gap-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>客户名称</FormLabel>
                  <FormControl>
                    <Input placeholder='必填' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='industry'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>行业</FormLabel>
                  <FormControl>
                    <Input placeholder='例如 软件、零售、制造' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='is_returning'
              render={({ field }) => (
                <FormItem className='flex items-center gap-2 col-span-1'>
                  <FormControl>
                    <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                  </FormControl>
                  <FormLabel className='!m-0'>回头客户</FormLabel>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='price_sensitive'
              render={({ field }) => (
                <FormItem className='flex items-center gap-2 col-span-1'>
                  <FormControl>
                    <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                  </FormControl>
                  <FormLabel className='!m-0'>价格敏感</FormLabel>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>联系人</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_phone'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>电话</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_email'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>邮箱</FormLabel>
                  <FormControl>
                    <Input type='email' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter className='col-span-2'>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={busy}>
                取消
              </Button>
              <Button type='submit' disabled={busy}>
                {busy ? '保存中…' : '保存'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: build 验证**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```
Expected: succeed.

如果 `@/components/ui/textarea` / `@/components/ui/checkbox` 不存在，用 shadcn CLI 生成：
```bash
cd apps/web
pnpm dlx shadcn@latest add textarea checkbox
```

- [ ] **Step 3: 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/customers/components/customer-form-dialog.tsx
# If shadcn generated new components, include them:
git add apps/web/src/components/ui/textarea.tsx apps/web/src/components/ui/checkbox.tsx 2>/dev/null || true
git commit -m "$(cat <<'EOF'
feat(web): customer create/edit dialog with zod-validated form

Shared component handles both create and edit modes; empty strings are
coerced to null before hitting the API so the DB stores NULL rather
than empty string. Mutation errors surface via the hook's toast so the
dialog keeps its form state and the user can retry.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 客户详情页 + 编辑

**Files:**
- Create: `apps/web/src/routes/_authenticated/customers/$id.tsx`
- Create: `apps/web/src/features/customers/detail.tsx`

- [ ] **Step 1: 路由**

Create `apps/web/src/routes/_authenticated/customers/$id.tsx`:
```tsx
import { createFileRoute } from '@tanstack/react-router'
import { CustomerDetail } from '@/features/customers/detail'
import { customerDetailQueryOptions } from '@/features/customers/hooks'

export const Route = createFileRoute('/_authenticated/customers/$id')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(customerDetailQueryOptions(params.id)),
  component: CustomerDetail,
})
```

- [ ] **Step 2: 详情组件**

Create `apps/web/src/features/customers/detail.tsx`:
```tsx
import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useCustomer } from './hooks'
import { CustomerFormDialog } from './components/customer-form-dialog'

const route = getRouteApi('/_authenticated/customers/$id')

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='grid grid-cols-4 gap-2 py-1'>
      <dt className='col-span-1 text-sm text-muted-foreground'>{label}</dt>
      <dd className='col-span-3 text-sm'>{value || '—'}</dd>
    </div>
  )
}

export function CustomerDetail() {
  const { id } = route.useParams()
  const { data, isLoading } = useCustomer(id)
  const [editOpen, setEditOpen] = useState(false)

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/customers'>← 返回列表</Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        {data && (
          <Card>
            <CardHeader className='flex flex-row items-center justify-between'>
              <CardTitle className='text-2xl'>{data.name}</CardTitle>
              <Button onClick={() => setEditOpen(true)}>编辑</Button>
            </CardHeader>
            <CardContent>
              <dl className='divide-y'>
                <Field label='行业' value={data.industry} />
                <Field label='回头客户' value={data.is_returning ? '是' : '否'} />
                <Field label='价格敏感' value={data.price_sensitive ? '是' : '否'} />
                <Field label='联系人' value={data.contact_name} />
                <Field label='电话' value={data.contact_phone} />
                <Field label='邮箱' value={data.contact_email} />
                <Field label='备注' value={data.notes} />
                <Field label='创建时间' value={new Date(data.created_at).toLocaleString()} />
                <Field label='更新时间' value={new Date(data.updated_at).toLocaleString()} />
              </dl>
            </CardContent>
          </Card>
        )}
      </Main>
      {data && (
        <CustomerFormDialog mode='edit' open={editOpen} onOpenChange={setEditOpen} initial={data} />
      )}
    </>
  )
}
```

- [ ] **Step 3: build + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
git add apps/web/src/routes/_authenticated/customers/\$id.tsx apps/web/src/features/customers/detail.tsx
git commit -m "$(cat <<'EOF'
feat(web): customer detail page with inline edit dialog

Route /_authenticated/customers/:id prefetches detail via router loader.
Detail view renders all fields in a card + dl/dt/dd grid. Edit button
opens the shared CustomerFormDialog in 'edit' mode; after save the
update mutation invalidates the list cache and seeds the detail cache
so the page re-renders with fresh data without a network round trip.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 字典 — 国家（admin 可编辑）

**Files:**
- Create: `apps/web/src/routes/_authenticated/catalog/countries.tsx`
- Create: `apps/web/src/features/catalog/countries.tsx`
- Create: `apps/web/src/features/catalog/components/countries-table.tsx`
- Create: `apps/web/src/features/catalog/components/country-edit-drawer.tsx`

- [ ] **Step 1: 路由 + beforeLoad 角色门卫**

Create `apps/web/src/routes/_authenticated/catalog/countries.tsx`:
```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQueryOptions } from '@/features/auth/hooks'
import { countriesQueryOptions } from '@/features/catalog/hooks'
import { CatalogCountries } from '@/features/catalog/countries'

export const Route = createFileRoute('/_authenticated/catalog/countries')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loader: ({ context }) =>
    context.queryClient.ensureQueryData(countriesQueryOptions(true)),
  component: CatalogCountries,
})
```

注意：countries 这一页带 `include_disabled=true` 让 admin 看到全部，包括被禁用的。普通角色走不到这里（beforeLoad 已拦）。

- [ ] **Step 2: 表格组件**

Create `apps/web/src/features/catalog/components/countries-table.tsx`:
```tsx
import { useMemo, useState } from 'react'
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Country } from '../types'

interface Props {
  data: Country[]
  onEdit: (row: Country) => void
}

export function CountriesTable({ data, onEdit }: Props) {
  const [filter, setFilter] = useState('')

  const columns = useMemo<ColumnDef<Country>[]>(
    () => [
      { accessorKey: 'code', header: 'ISO' },
      { accessorKey: 'name_zh', header: '中文名' },
      { accessorKey: 'name_en', header: '英文名' },
      {
        accessorKey: 'is_madrid_member',
        header: 'Madrid',
        cell: ({ getValue }) =>
          getValue<boolean>() ? <Badge>成员</Badge> : <span className='text-muted-foreground'>—</span>,
      },
      {
        accessorKey: 'default_acceptance_days',
        header: '受理天数',
        cell: ({ getValue }) => getValue<number | null>() ?? '—',
      },
      {
        accessorKey: 'default_registration_months',
        header: '注册月数',
        cell: ({ getValue }) => getValue<number | null>() ?? '—',
      },
      {
        accessorKey: 'enabled',
        header: '启用',
        cell: ({ getValue }) =>
          getValue<boolean>() ? <Badge variant='secondary'>启用</Badge> : <Badge variant='outline'>停用</Badge>,
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <Button size='sm' variant='ghost' onClick={() => onEdit(row.original)}>
            编辑
          </Button>
        ),
      },
    ],
    [onEdit]
  )

  const table = useReactTable({
    data,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    globalFilterFn: (row, _col, val: string) => {
      const v = val.toLowerCase()
      const r = row.original
      return [r.code, r.name_zh, r.name_en].some((f) => f.toLowerCase().includes(v))
    },
  })

  return (
    <div className='flex flex-col gap-3'>
      <Input
        placeholder='按 ISO / 中文名 / 英文名搜索'
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className='max-w-xs'
      />
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => (
                  <TableHead key={h.id}>
                    {h.isPlaceholder ? null : flexRender(h.column.columnDef.header, h.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  无数据
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: 编辑 drawer**

Create `apps/web/src/features/catalog/components/country-edit-drawer.tsx`:
```tsx
import { useEffect } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import type { Country } from '../types'
import { useUpdateCountry } from '../hooks'

const schema = z.object({
  name_zh: z.string().min(1, '中文名不能为空').max(100),
  name_en: z.string().min(1, '英文名不能为空').max(200),
  is_madrid_member: z.boolean(),
  default_acceptance_days: z
    .union([z.coerce.number().int().min(0).max(3650), z.literal('')])
    .optional(),
  default_registration_months: z
    .union([z.coerce.number().int().min(0).max(240), z.literal('')])
    .optional(),
  requires_notarization: z.boolean(),
  notes_zh: z.string().max(1000).optional().or(z.literal('')),
  notes_en: z.string().max(1000).optional().or(z.literal('')),
  sort_order: z.coerce.number().int().min(0).max(10_000),
  enabled: z.boolean(),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  country: Country | null
}

export function CountryEditDrawer({ open, onOpenChange, country }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name_zh: '',
      name_en: '',
      is_madrid_member: false,
      default_acceptance_days: '',
      default_registration_months: '',
      requires_notarization: false,
      notes_zh: '',
      notes_en: '',
      sort_order: 0,
      enabled: true,
    },
  })
  const updateMut = useUpdateCountry()

  useEffect(() => {
    if (!open || !country) return
    form.reset({
      name_zh: country.name_zh,
      name_en: country.name_en,
      is_madrid_member: country.is_madrid_member,
      default_acceptance_days: country.default_acceptance_days ?? '',
      default_registration_months: country.default_registration_months ?? '',
      requires_notarization: country.requires_notarization,
      notes_zh: country.notes_zh ?? '',
      notes_en: country.notes_en ?? '',
      sort_order: country.sort_order,
      enabled: country.enabled,
    })
  }, [open, country, form])

  const onSubmit = form.handleSubmit(async (values) => {
    if (!country) return
    await updateMut
      .mutateAsync({
        id: country.id,
        body: {
          name_zh: values.name_zh,
          name_en: values.name_en,
          is_madrid_member: values.is_madrid_member,
          default_acceptance_days:
            values.default_acceptance_days === '' ? null : Number(values.default_acceptance_days),
          default_registration_months:
            values.default_registration_months === '' ? null : Number(values.default_registration_months),
          requires_notarization: values.requires_notarization,
          notes_zh: values.notes_zh || null,
          notes_en: values.notes_en || null,
          sort_order: values.sort_order,
          enabled: values.enabled,
        },
      })
      .then(() => onOpenChange(false))
      .catch(() => {
        /* toast shown inside hook */
      })
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-xl overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>{country ? `编辑国家 · ${country.code}` : '编辑国家'}</SheetTitle>
          <SheetDescription>修改后点击保存。ISO 代码不可更改。</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='name_zh'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>中文名</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='name_en'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>英文名</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='default_acceptance_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>默认受理天数</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='default_registration_months'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>默认注册月数</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='sort_order'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>排序权重</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='is_madrid_member'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>Madrid 成员国</FormLabel>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='requires_notarization'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>需要公证</FormLabel>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>启用</FormLabel>
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='notes_zh'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>中文备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes_en'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>英文备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SheetFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={updateMut.isPending}>
                取消
              </Button>
              <Button type='submit' disabled={updateMut.isPending}>
                {updateMut.isPending ? '保存中…' : '保存'}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
```

- [ ] **Step 4: 页面组件**

Create `apps/web/src/features/catalog/countries.tsx`:
```tsx
import { useState } from 'react'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import type { Country } from './types'
import { useCountries } from './hooks'
import { CountriesTable } from './components/countries-table'
import { CountryEditDrawer } from './components/country-edit-drawer'

export function CatalogCountries() {
  const { data = [], isLoading } = useCountries(true)
  const [edit, setEdit] = useState<Country | null>(null)

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>国家字典</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>国家字典</h2>
          <p className='text-muted-foreground'>管理员可编辑国家的双语名称、默认时效、Madrid 身份等。</p>
        </div>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        <CountriesTable data={data} onEdit={(row) => setEdit(row)} />
      </Main>
      <CountryEditDrawer open={!!edit} onOpenChange={(o) => !o && setEdit(null)} country={edit} />
    </>
  )
}
```

- [ ] **Step 5: 确保 `@/components/ui/sheet` 存在**

```bash
cd /Users/adam/workspace/github/trademark-admin
ls apps/web/src/components/ui/sheet.tsx || pnpm -C apps/web dlx shadcn@latest add sheet
```
如果已存在就跳过。

- [ ] **Step 6: build + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
git add apps/web/src/routes/_authenticated/catalog/countries.tsx apps/web/src/features/catalog/countries.tsx apps/web/src/features/catalog/components/
# include sheet.tsx if it got generated:
git add apps/web/src/components/ui/sheet.tsx 2>/dev/null || true
git commit -m "$(cat <<'EOF'
feat(web): admin-only catalog/countries page with edit drawer

Route's beforeLoad reads the cached /auth/me and redirects non-admins to
/403. Main view is a client-filtered TanStack Table over all countries
(admin sees include_disabled=true). Row action opens a Sheet-based edit
form covering bilingual names, default timings, Madrid + notarization
booleans, sort order, enabled toggle. Save calls useUpdateCountry which
invalidates the countries cache on success.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: 字典 — 尼斯分类（只读）

**Files:**
- Create: `apps/web/src/routes/_authenticated/catalog/nice-categories.tsx`
- Create: `apps/web/src/features/catalog/nice-categories.tsx`
- Create: `apps/web/src/features/catalog/components/nice-categories-table.tsx`

尼斯分类也仅对 admin 暴露（在 sidebar 里只有 admin 看得到，beforeLoad 强化）。但读接口本身 `GET /catalog/nice-categories` 对所有 authed 用户放开 —— 其他角色直接访问 URL 会被 beforeLoad 跳到 /403，与 Plan 4 Definition of Done 的"管理员维护字典"一致。

- [ ] **Step 1: 路由**

Create `apps/web/src/routes/_authenticated/catalog/nice-categories.tsx`:
```tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQueryOptions } from '@/features/auth/hooks'
import { niceCategoriesQueryOptions } from '@/features/catalog/hooks'
import { CatalogNiceCategories } from '@/features/catalog/nice-categories'

export const Route = createFileRoute('/_authenticated/catalog/nice-categories')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loader: ({ context }) =>
    context.queryClient.ensureQueryData(niceCategoriesQueryOptions),
  component: CatalogNiceCategories,
})
```

- [ ] **Step 2: 表格**

Create `apps/web/src/features/catalog/components/nice-categories-table.tsx`:
```tsx
import { useState } from 'react'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { NiceCategory } from '../types'

interface Props {
  data: NiceCategory[]
}

export function NiceCategoriesTable({ data }: Props) {
  const [filter, setFilter] = useState('')

  const filtered = data.filter((r) => {
    if (!filter.trim()) return true
    const f = filter.toLowerCase()
    return (
      String(r.code).includes(f) ||
      r.name_zh.toLowerCase().includes(f) ||
      r.name_en.toLowerCase().includes(f)
    )
  })

  return (
    <div className='flex flex-col gap-3'>
      <Input
        placeholder='按代码或名称搜索'
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className='max-w-xs'
      />
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-20'>代码</TableHead>
              <TableHead>中文名</TableHead>
              <TableHead>英文名</TableHead>
              <TableHead>中文描述</TableHead>
              <TableHead>英文描述</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length ? (
              filtered.map((r) => (
                <TableRow key={r.code}>
                  <TableCell className='font-mono'>{r.code}</TableCell>
                  <TableCell>{r.name_zh}</TableCell>
                  <TableCell>{r.name_en}</TableCell>
                  <TableCell className='max-w-xs truncate' title={r.description_zh ?? ''}>
                    {r.description_zh ?? '—'}
                  </TableCell>
                  <TableCell className='max-w-xs truncate' title={r.description_en ?? ''}>
                    {r.description_en ?? '—'}
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={5} className='h-24 text-center'>
                  无数据
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: 页面组件**

Create `apps/web/src/features/catalog/nice-categories.tsx`:
```tsx
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useNiceCategories } from './hooks'
import { NiceCategoriesTable } from './components/nice-categories-table'

export function CatalogNiceCategories() {
  const { data = [], isLoading } = useNiceCategories()
  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>尼斯分类</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>尼斯分类</h2>
          <p className='text-muted-foreground'>国际商标尼斯分类 45 项，仅查看。</p>
        </div>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        <NiceCategoriesTable data={data} />
      </Main>
    </>
  )
}
```

- [ ] **Step 4: build + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
git add apps/web/src/routes/_authenticated/catalog/nice-categories.tsx apps/web/src/features/catalog/nice-categories.tsx apps/web/src/features/catalog/components/nice-categories-table.tsx
git commit -m "$(cat <<'EOF'
feat(web): admin-only read-only catalog/nice-categories page

Plain Chinese + English grid over the 45 Nice classes, with local filter
on code/zh/en columns. No edit path — Plan 4 DoD marks this dictionary
as seed-only for MVP.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: MSW 集成测试 —— 客户创建 → 列表闭环

**Files:**
- Modify: `apps/web/src/test-utils/msw/handlers.ts`
- Create: `apps/web/src/features/customers/customers.integration.test.tsx`

- [ ] **Step 1: 扩展 MSW handlers**

Open `apps/web/src/test-utils/msw/handlers.ts`. Inside `defaultHandlers` append the new routes. The handlers keep the same `loggedIn` module variable so tests start unauthenticated by default.

Replace `apps/web/src/test-utils/msw/handlers.ts`:
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

// In-memory customers store used across requests within a single test.
let customers: Array<{
  id: string
  name: string
  industry: string | null
  is_returning: boolean
  price_sensitive: boolean
  contact_name: string | null
  contact_phone: string | null
  contact_email: string | null
  notes: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []

function randomUUID() {
  // minimal RFC-4122 v4; browser polyfills have crypto.randomUUID too.
  const s = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
  return s
}

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

  // ---- customers ----
  http.get('/api/v1/customers', ({ request }) => {
    const url = new URL(request.url)
    const q = (url.searchParams.get('q') ?? '').toLowerCase()
    const page = Number(url.searchParams.get('page') ?? 1)
    const size = Number(url.searchParams.get('page_size') ?? 20)

    const filtered = q
      ? customers.filter(
          (c) =>
            c.name.toLowerCase().includes(q) ||
            (c.industry ?? '').toLowerCase().includes(q)
        )
      : customers
    const start = (page - 1) * size
    return HttpResponse.json({
      items: filtered.slice(start, start + size),
      page,
      page_size: size,
      total: filtered.length,
    })
  }),
  http.post('/api/v1/customers', async ({ request }) => {
    const body = (await request.json()) as Partial<{
      name: string
      industry: string | null
      is_returning: boolean
      price_sensitive: boolean
      contact_name: string | null
      contact_phone: string | null
      contact_email: string | null
      notes: string | null
    }>
    if (!body.name) {
      return HttpResponse.json({ code: 'ERR_INVALID_BODY', message: 'name required' }, { status: 400 })
    }
    if (customers.some((c) => c.name === body.name)) {
      return HttpResponse.json({ code: 'ERR_DUPLICATE_NAME', message: 'duplicate' }, { status: 409 })
    }
    const now = new Date().toISOString()
    const row = {
      id: randomUUID(),
      name: body.name,
      industry: body.industry ?? null,
      is_returning: body.is_returning ?? false,
      price_sensitive: body.price_sensitive ?? false,
      contact_name: body.contact_name ?? null,
      contact_phone: body.contact_phone ?? null,
      contact_email: body.contact_email ?? null,
      notes: body.notes ?? null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    customers = [row, ...customers]
    return HttpResponse.json(row, { status: 201 })
  }),
  http.get('/api/v1/customers/:id', ({ params }) => {
    const row = customers.find((c) => c.id === params.id)
    if (!row) {
      return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    }
    return HttpResponse.json(row)
  }),

  // ---- catalog minimal handlers to satisfy sidebar / 403 cases ----
  http.get('/api/v1/catalog/countries', () => {
    return HttpResponse.json({ items: [] })
  }),
  http.get('/api/v1/catalog/nice-categories', () => {
    return HttpResponse.json({ items: [] })
  }),
]

export function resetMswState() {
  loggedIn = false
  customers = []
}
```

注意：`resetMswState` 现在也清空 in-memory customers 列表；Plan 3 测试也调了这个函数，所以向后兼容。

- [ ] **Step 2: 写集成测试**

Create `apps/web/src/features/customers/customers.integration.test.tsx`:
```tsx
import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
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
import { meQueryOptions } from '@/features/auth/hooks'
import { SignIn } from '@/features/auth/sign-in'
import { Customers } from '@/features/customers'

function buildRouter() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const rootRoute = createRootRoute({ component: () => <Outlet /> })
  const signInRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/sign-in',
    validateSearch: (s: Record<string, unknown>) => ({ redirect: (s.redirect as string) ?? '' }),
    component: SignIn,
  })
  const customersRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/customers',
    validateSearch: (s: Record<string, unknown>) => ({
      q: (s.q as string | undefined) ?? undefined,
      page: s.page ? Number(s.page) : undefined,
      page_size: s.page_size ? Number(s.page_size) : undefined,
    }),
    beforeLoad: async () => {
      try {
        await queryClient.ensureQueryData(meQueryOptions)
      } catch {
        throw redirect({ to: '/sign-in', search: { redirect: '/customers' } })
      }
    },
    component: Customers,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([signInRoute, customersRoute]),
    history: createMemoryHistory({ initialEntries: ['/customers'] }),
    context: { queryClient },
  })
  return { router, queryClient }
}

describe('customers integration', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'bypass' })
  })

  beforeEach(() => {
    resetMswState()
    useAuthStore.getState().auth.reset()
  })

  afterAll(() => {
    worker.stop()
  })

  it('guard → sign-in → customers list → create → list shows row', async () => {
    const { router, queryClient } = buildRouter()
    const { getByRole, getByLabelText, getByText } = render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Initial guard redirects /customers → /sign-in
    await expect.element(getByRole('button', { name: '登录' })).toBeInTheDocument()

    await userEvent.type(getByLabelText('邮箱'), 'admin@example.com')
    await userEvent.type(getByLabelText('密码'), 'change-me-on-first-login')
    await userEvent.click(getByRole('button', { name: '登录' }))

    // Land on customers list, empty.
    await expect.element(getByText('客户档案')).toBeInTheDocument()
    await expect.element(getByText('暂无客户')).toBeInTheDocument()

    // Open create dialog.
    await userEvent.click(getByRole('button', { name: '新建客户' }))
    await userEvent.type(getByLabelText('客户名称'), 'Smoke-Test-Acme')
    await userEvent.click(getByRole('button', { name: '保存' }))

    // Row appears in list.
    await expect.element(getByText('Smoke-Test-Acme')).toBeInTheDocument()
  })
})
```

注意：sign-in 登录成功后 `useLogin` 写 store + 缓存 /auth/me，router 的 `customersRoute.beforeLoad` 在下一次导航时读缓存成功；可是目前 sign-in 成功后 **没有自动导航**。检查 `SignIn` 组件：`useLogin` 的 onSuccess 会不会 `navigate`？Plan 3 的登录页是通过 Top-level `useSearch` 拿 `redirect` 参数并在 onSuccess 里 navigate。如果此处登录后没跳回 `/customers`，集成测试会挂在 sign-in 页。

验证方式：在 `apps/web/src/features/auth/sign-in/components/user-auth-form.tsx` 中查 `onSuccess` — 正常该在成功后 `navigate({ to: redirect || '/' })`。若该行逻辑用的是 `navigate({ to: '/' })`（硬编码首页），则集成测试需要在测试路由树里多挂一个 `/` 路由重定向到 `/customers`，或者让 SignIn 组件尊重 search.redirect。

如果是后者，先修好 sign-in 组件（让它跳到 search.redirect 或默认 `/`），再写测试。本 task 不强行改 SignIn —— 如果发现 SignIn 不做 redirect，可先在集成测试里 router 初始 `/customers` 让 beforeLoad 触发，然后登录成功后手动在测试中 `await router.navigate({ to: '/customers' })` 模拟。

- [ ] **Step 3: 跑集成测试**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/features/customers/customers.integration.test.tsx
```
Expected: 1 PASS.

常见问题：
- `Welcome` 找不到 —— SignIn 登录成功后没跳转。查 `sign-in/components/user-auth-form.tsx` 的 `useLogin` 成功分支。
- `暂无客户` 字符被多处命中 —— 前面描述里也有 "客户"，用更具体的文本定位（例如 `getByText(/暂无客户/)`）。
- 创建 dialog 打不开 —— `Dialog` 组件在 Radix 下需要 portal，检查 `vitest-browser-react` 的 render 支持。shadcn Dialog 默认 portal 到 document.body，`getByRole` 应能找到。

- [ ] **Step 4: 全量测试 + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run
pnpm -C apps/web build
pnpm -C apps/web lint
git add apps/web/src/test-utils/msw/handlers.ts apps/web/src/features/customers/customers.integration.test.tsx
git commit -m "$(cat <<'EOF'
test(web): integration test covering customers create -> list round trip

MSW handlers grow in-memory /customers CRUD + minimal catalog stubs.
resetMswState now also clears the customer store so each test starts
empty. The new test walks: guarded /customers -> sign-in -> successful
login -> customers list empty -> new customer dialog -> save -> list
shows the new row.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Plan 5 Definition of Done

1. ✅ `pnpm -C apps/web build` 成功
2. ✅ `pnpm -C apps/web test --run` 全绿（含 Plan 3 既有测试 + 新的客户集成测试）
3. ✅ `pnpm -C apps/web lint` 无错误
4. ✅ Sidebar: admin 看到 Dashboard / 客户 / 字典（国家 + 尼斯分类）/ 系统；其他角色看不到字典分组
5. ✅ 未登录访问任何 `/customers` 或 `/catalog/*` 自动跳 `/sign-in`
6. ✅ 非 admin 用户访问 `/catalog/countries` 或 `/catalog/nice-categories` 跳 `/403`
7. ✅ 业务员登录 → /customers 列表只看自己创建的（后端已保证）
8. ✅ 新建客户：表单校验中文化，同名返回 409 → "已存在同名客户" toast
9. ✅ 客户详情：Card 展示所有字段；编辑按钮打开同一 Dialog（edit 模式）
10. ✅ 国家字典：admin 看到 61 条；编辑 drawer 保存后列表刷新
11. ✅ 尼斯分类：admin 看到 45 条，只读

## 下一步

Plan 5 完成后进入 **Plan 6（成本模板 + 计算引擎）**：
- 后端 migration 增加 `pricing_entries`（immutable 版本化，country × registration_method × service_tier × fee_item）
- `/api/v1/pricing-entries` CRUD + 历史查询 + deprecate
- 纯函数计算引擎 `internal/pricing/calc.go`（输入 quotation draft → 输出条目明细 + 总价 + 签名）
- 前端 `/pricing/*` 页面（reviewer/admin 可见）：二维表格编辑 + 历史时间线
