# Plan 7: 前端定价 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Plan 6 交付的 `/api/v1/pricing-entries` 接到前端：reviewer/admin 可见的 `/pricing` 矩阵视图（country × service_tier 两维，展开看 fee_item 条目），编辑 drawer（admin 写），历史时间线，sidebar 新增"定价"入口（reviewer/admin 可见）。

**Architecture:** 复用 Plan 5 的 `features/*/hooks + components + routes` 分层约定。因为定价表是全量小数据（61 countries × 3 tiers × N fee_items，典型数百行），走客户端过滤而非服务端分页；列表页是一个可切换 country/tier 的 2D 展开表。写操作交给 admin-only drawer，非 admin 连编辑按钮都不见（复用 useMe + role 判断）。历史走点击条目弹 Sheet → 时间线。`/pricing` 路由 `beforeLoad` gate 要求 role ∈ {reviewer, admin}，否则 redirect /403。

**Tech Stack:** React 19 + Vite + TanStack Router + TanStack Query + @tanstack/react-table + react-hook-form + zod + shadcn/ui + Sonner。无新依赖。

---

## File Structure

### Create

**类型 + hooks**
- `apps/web/src/features/pricing/types.ts`
- `apps/web/src/features/pricing/hooks/use-pricing-entries.ts`
- `apps/web/src/features/pricing/hooks/use-pricing-history.ts`
- `apps/web/src/features/pricing/hooks/use-pricing-mutations.ts`
- `apps/web/src/features/pricing/hooks/index.ts`

**页面 + 组件**
- `apps/web/src/routes/_authenticated/pricing/index.tsx`
- `apps/web/src/features/pricing/index.tsx`
- `apps/web/src/features/pricing/components/pricing-matrix.tsx` — 2D 主视图
- `apps/web/src/features/pricing/components/pricing-entry-drawer.tsx` — admin 编辑 drawer
- `apps/web/src/features/pricing/components/pricing-history-sheet.tsx` — 历史时间线

**测试 + MSW**
- `apps/web/src/features/pricing/pricing.integration.test.tsx`（最后一个 task）

### Modify

- `apps/web/src/components/layout/data/sidebar-data.ts` — 给 reviewer + admin 加 "定价" 条目
- `apps/web/src/test-utils/msw/handlers.ts` — 加 /pricing-entries + history + POST stubs

---

## Task 1: TS 类型

**Files:**
- Create: `apps/web/src/features/pricing/types.ts`

- [ ] **Step 1: 类型定义**

```ts
// apps/web/src/features/pricing/types.ts

export type ServiceTier = 'basic' | 'standard' | 'premium'

export const SERVICE_TIERS: ServiceTier[] = ['basic', 'standard', 'premium']

// Friendly Chinese labels for tiers.
export const SERVICE_TIER_LABEL_ZH: Record<ServiceTier, string> = {
  basic: '基础',
  standard: '标准',
  premium: '高级',
}

export interface PricingEntry {
  id: string
  country_id: string
  service_tier: ServiceTier
  fee_item: string
  amount_cny_cents: number
  notes?: string | null
  effective_from: string // YYYY-MM-DD
  effective_to?: string | null // YYYY-MM-DD or null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateOrReplacePricingRequest {
  country_id: string
  service_tier: ServiceTier
  fee_item: string
  amount_cny_cents: number
  notes?: string | null
  effective_from: string
}

export interface ListEnvelope<T> {
  items: T[]
}
```

- [ ] **Step 2: build**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
```

Expected: succeed.

- [ ] **Step 3: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/pricing/types.ts
git commit -m "$(cat <<'EOF'
feat(web): pricing TS types mirroring Go DTO + tier labels

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: TanStack Query hooks

**Files:**
- Create: `apps/web/src/features/pricing/hooks/{use-pricing-entries,use-pricing-history,use-pricing-mutations,index}.ts`

- [ ] **Step 1: use-pricing-entries.ts**

```ts
// apps/web/src/features/pricing/hooks/use-pricing-entries.ts
import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { ListEnvelope, PricingEntry, ServiceTier } from '../types'

export const PRICING_QUERY_KEY = ['pricing'] as const

interface ListArgs {
  country_id?: string
  service_tier?: ServiceTier
}

export const pricingListQueryOptions = (args: ListArgs = {}) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'list', args] as const,
    queryFn: async (): Promise<PricingEntry[]> => {
      const res = await api.get<ListEnvelope<PricingEntry>>('/pricing-entries', {
        params: {
          country_id: args.country_id || undefined,
          service_tier: args.service_tier || undefined,
        },
      })
      return res.data.items
    },
    staleTime: 60 * 1000,
  })

export function usePricingList(args: ListArgs = {}) {
  return useQuery(pricingListQueryOptions(args))
}
```

- [ ] **Step 2: use-pricing-history.ts**

```ts
// apps/web/src/features/pricing/hooks/use-pricing-history.ts
import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { ListEnvelope, PricingEntry, ServiceTier } from '../types'
import { PRICING_QUERY_KEY } from './use-pricing-entries'

interface HistoryArgs {
  country_id: string
  service_tier: ServiceTier
  fee_item: string
}

export const pricingHistoryQueryOptions = (args: HistoryArgs) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'history', args] as const,
    queryFn: async (): Promise<PricingEntry[]> => {
      const res = await api.get<ListEnvelope<PricingEntry>>('/pricing-entries/history', {
        params: args,
      })
      return res.data.items
    },
    enabled: !!args.country_id && !!args.service_tier && !!args.fee_item,
  })

export function usePricingHistory(args: HistoryArgs) {
  return useQuery(pricingHistoryQueryOptions(args))
}
```

- [ ] **Step 3: use-pricing-mutations.ts**

```ts
// apps/web/src/features/pricing/hooks/use-pricing-mutations.ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type { CreateOrReplacePricingRequest, PricingEntry } from '../types'
import { PRICING_QUERY_KEY } from './use-pricing-entries'

function mapPricingError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_INVALID_TIER') return '不支持的服务级别'
    if (code === 'ERR_ALREADY_DEPRECATED') return '该条目已被废止'
    if (err.response?.status === 403) return '没有权限修改定价'
  }
  return '保存失败，请稍后重试'
}

export function useCreateOrReplacePricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateOrReplacePricingRequest): Promise<PricingEntry> => {
      const res = await api.post<PricingEntry>('/pricing-entries', body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('定价已保存')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}

export function useDeprecatePricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; effective_to?: string }): Promise<PricingEntry> => {
      const res = await api.post<PricingEntry>(`/pricing-entries/${args.id}/deprecate`, {
        effective_to: args.effective_to,
      })
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('定价已废止')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}
```

- [ ] **Step 4: index.ts**

```ts
// apps/web/src/features/pricing/hooks/index.ts
export * from './use-pricing-entries'
export * from './use-pricing-history'
export * from './use-pricing-mutations'
```

- [ ] **Step 5: build + 提交**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
git add apps/web/src/features/pricing/hooks/
git commit -m "$(cat <<'EOF'
feat(web): TanStack Query hooks for pricing list/history/mutations

pricingListQueryOptions accepts optional country_id + service_tier
filters. History hook is disabled until all three key fields are set.
Mutations share error-code mapping (ERR_INVALID_TIER /
ERR_ALREADY_DEPRECATED / 403) to dedicated Chinese toasts.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Sidebar 加"定价"入口 — reviewer + admin 可见

**Files:**
- Modify: `apps/web/src/components/layout/data/sidebar-data.ts`

- [ ] **Step 1: 扩展 navGroupsFor**

打开 `apps/web/src/components/layout/data/sidebar-data.ts`。当前实现是：
```ts
function navGroupsFor(role: AuthUser['role']): NavGroup[] {
  const base: NavGroup[] = [ /* Dashboard + Customers */ ]
  if (role === 'admin') { /* 字典 group */ }
  base.push({ /* 系统 */ })
  return base
}
```

在 admin 字典组之前（或之后，位置无所谓）加一个 reviewer+admin 都可见的 "业务" 组，里面放 "定价"：

具体改法 — 找到 `const base: NavGroup[] = [` 所在的 "主导航" 组。在它后面、`if (role === 'admin')` 之前，加：

```ts
  if (role === 'reviewer' || role === 'admin') {
    base.push({
      title: '业务',
      items: [
        {
          title: '定价',
          url: '/pricing',
          icon: Calculator,
        },
      ],
    })
  }
```

在文件顶部 `import { ... } from 'lucide-react'` 里加 `Calculator`（与其他图标排序即可）。

- [ ] **Step 2: build + 测试**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
pnpm -C apps/web test --run
```

Expected: build 绿；search-provider.test 依然绿（其 useMe mock 返回 admin，所以新增的"业务"组对 admin 也可见，原有断言是 `仪表盘 / 客户 / 国家 / 个人设置 / 外观`，不断言"业务"组存在，所以不 break）。

- [ ] **Step 3: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/components/layout/data/sidebar-data.ts
git commit -m "$(cat <<'EOF'
feat(web): sidebar "业务 > 定价" group visible to reviewer + admin

Salesperson still sees none of this. Follows the same role-aware
buildSidebarData pattern used for the admin-only 字典 group.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 2D 矩阵视图组件

**Files:**
- Create: `apps/web/src/features/pricing/components/pricing-matrix.tsx`

- [ ] **Step 1: 组件**

矩阵按 fee_item 行 × service_tier 列，给定一个 country_id。每格显示金额（CNY，格式化），点击打开编辑 drawer（如果是 admin）或历史 sheet（非 admin 或管理员右键）。MVP 直接每格一个双击进编辑、单击看历史。

```tsx
// apps/web/src/features/pricing/components/pricing-matrix.tsx
import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type {
  PricingEntry,
  ServiceTier,
} from '../types'
import { SERVICE_TIERS, SERVICE_TIER_LABEL_ZH } from '../types'

export interface MatrixCell {
  feeItem: string
  byTier: Partial<Record<ServiceTier, PricingEntry>>
}

// toMatrix pivots a flat list of active PricingEntries into rows by
// fee_item, columns by service_tier. Undefined cells mean no active
// entry for that dimension — render as "—".
export function toMatrix(entries: PricingEntry[]): MatrixCell[] {
  const byFeeItem = new Map<string, MatrixCell>()
  for (const e of entries) {
    if (e.effective_to) continue // defensive: backend should already filter
    let cell = byFeeItem.get(e.fee_item)
    if (!cell) {
      cell = { feeItem: e.fee_item, byTier: {} }
      byFeeItem.set(e.fee_item, cell)
    }
    cell.byTier[e.service_tier] = e
  }
  return Array.from(byFeeItem.values()).sort((a, b) =>
    a.feeItem.localeCompare(b.feeItem)
  )
}

export function formatCNY(cents: number): string {
  const yuan = cents / 100
  return '¥' + yuan.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

interface Props {
  entries: PricingEntry[]
  canEdit: boolean
  onEditCell: (feeItem: string, tier: ServiceTier, current?: PricingEntry) => void
  onOpenHistory: (feeItem: string, tier: ServiceTier) => void
}

export function PricingMatrix({ entries, canEdit, onEditCell, onOpenHistory }: Props) {
  const matrix = useMemo(() => toMatrix(entries), [entries])

  if (matrix.length === 0) {
    return <p className='text-sm text-muted-foreground'>该国家暂无定价条目。{canEdit && ' 点击右上方“新增条目”开始添加。'}</p>
  }

  return (
    <div className='overflow-hidden rounded-md border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='w-64'>费用项</TableHead>
            {SERVICE_TIERS.map((t) => (
              <TableHead key={t}>{SERVICE_TIER_LABEL_ZH[t]}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {matrix.map((row) => (
            <TableRow key={row.feeItem}>
              <TableCell className='font-mono text-sm'>{row.feeItem}</TableCell>
              {SERVICE_TIERS.map((t) => {
                const entry = row.byTier[t]
                return (
                  <TableCell key={t}>
                    <div className='flex items-center gap-2'>
                      <span className={entry ? 'font-medium' : 'text-muted-foreground'}>
                        {entry ? formatCNY(entry.amount_cny_cents) : '—'}
                      </span>
                      {entry && (
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => onOpenHistory(row.feeItem, t)}
                          className='h-6 px-2 text-xs'
                        >
                          历史
                        </Button>
                      )}
                      {canEdit && (
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => onEditCell(row.feeItem, t, entry)}
                          className='h-6 px-2 text-xs'
                        >
                          {entry ? '修改' : '新增'}
                        </Button>
                      )}
                    </div>
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
```

- [ ] **Step 2: 快速编译（此时引用它的页面还没写，允许 build 不过。仍编译单文件检查语法）**

```bash
cd /Users/adam/workspace/github/trademark-admin
# 跳过 pnpm build — 整页在 Task 6 补
```

- [ ] **Step 3: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/pricing/components/pricing-matrix.tsx
git commit -m "$(cat <<'EOF'
feat(web): pricing matrix component — fee_item rows × service_tier cols

toMatrix pivots flat PricingEntry list into a sorted fee_item × tier
grid, excludes deprecated rows defensively, and formats amounts as
¥NNN.NN. Each cell has a 历史 button (always) and 修改/新增 button
(when canEdit). Empty cells render "—" so admins can see where to
add prices.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 编辑 drawer + 历史 sheet

**Files:**
- Create: `apps/web/src/features/pricing/components/pricing-entry-drawer.tsx`
- Create: `apps/web/src/features/pricing/components/pricing-history-sheet.tsx`

- [ ] **Step 1: drawer**

```tsx
// apps/web/src/features/pricing/components/pricing-entry-drawer.tsx
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
import { Button } from '@/components/ui/button'
import type { PricingEntry, ServiceTier } from '../types'
import { SERVICE_TIER_LABEL_ZH } from '../types'
import { useCreateOrReplacePricing, useDeprecatePricing } from '../hooks'

// amount in 元 (e.g. 120.50), converted to cents on submit
const schema = z.object({
  amount_cny_yuan: z.string().refine(
    (v) => /^\d+(\.\d{1,2})?$/.test(v) && Number(v) >= 0,
    { message: '请输入非负金额，最多两位小数' }
  ),
  effective_from: z.string().refine(
    (v) => /^\d{4}-\d{2}-\d{2}$/.test(v),
    { message: '格式必须是 YYYY-MM-DD' }
  ),
  notes: z.string().max(2000).optional().or(z.literal('')),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  feeItem: string
  serviceTier: ServiceTier
  current?: PricingEntry
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

export function PricingEntryDrawer({
  open,
  onOpenChange,
  countryId,
  feeItem,
  serviceTier,
  current,
}: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { amount_cny_yuan: '0', effective_from: todayISO(), notes: '' },
  })
  const createMut = useCreateOrReplacePricing()
  const deprecateMut = useDeprecatePricing()

  useEffect(() => {
    if (!open) return
    if (current) {
      form.reset({
        amount_cny_yuan: (current.amount_cny_cents / 100).toFixed(2),
        effective_from: todayISO(),
        notes: current.notes ?? '',
      })
    } else {
      form.reset({ amount_cny_yuan: '0', effective_from: todayISO(), notes: '' })
    }
  }, [open, current, form])

  const onSubmit = form.handleSubmit(async (values) => {
    const cents = Math.round(Number(values.amount_cny_yuan) * 100)
    await createMut
      .mutateAsync({
        country_id: countryId,
        service_tier: serviceTier,
        fee_item: feeItem,
        amount_cny_cents: cents,
        notes: values.notes || null,
        effective_from: values.effective_from,
      })
      .then(() => onOpenChange(false))
      .catch(() => { /* toast already shown */ })
  })

  const onDeprecate = async () => {
    if (!current) return
    await deprecateMut
      .mutateAsync({ id: current.id })
      .then(() => onOpenChange(false))
      .catch(() => { /* toast already shown */ })
  }

  const busy = createMut.isPending || deprecateMut.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-lg overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>
            {current ? '修改' : '新增'}定价 · {feeItem} · {SERVICE_TIER_LABEL_ZH[serviceTier]}
          </SheetTitle>
          <SheetDescription>
            {current
              ? '保存会生成一条新版本并自动废止当前生效版本。'
              : '填写金额与生效日期。'}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            <FormField
              control={form.control}
              name='amount_cny_yuan'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>金额（人民币元）</FormLabel>
                  <FormControl>
                    <Input inputMode='decimal' placeholder='0.00' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='effective_from'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>生效日期</FormLabel>
                  <FormControl>
                    <Input type='date' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SheetFooter className='flex flex-row items-center justify-between gap-2'>
              {current && (
                <Button
                  type='button'
                  variant='destructive'
                  onClick={onDeprecate}
                  disabled={busy}
                >
                  废止当前版本
                </Button>
              )}
              <div className='ms-auto flex gap-2'>
                <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={busy}>
                  取消
                </Button>
                <Button type='submit' disabled={busy}>
                  {busy ? '保存中…' : '保存'}
                </Button>
              </div>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
```

- [ ] **Step 2: history sheet**

```tsx
// apps/web/src/features/pricing/components/pricing-history-sheet.tsx
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import type { ServiceTier } from '../types'
import { SERVICE_TIER_LABEL_ZH } from '../types'
import { formatCNY } from './pricing-matrix'
import { usePricingHistory } from '../hooks'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  feeItem: string
  serviceTier: ServiceTier
}

export function PricingHistorySheet({
  open,
  onOpenChange,
  countryId,
  feeItem,
  serviceTier,
}: Props) {
  const { data = [], isLoading } = usePricingHistory({
    country_id: countryId,
    service_tier: serviceTier,
    fee_item: feeItem,
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-lg overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>
            价格历史 · {feeItem} · {SERVICE_TIER_LABEL_ZH[serviceTier]}
          </SheetTitle>
          <SheetDescription>
            每个版本对应一次"生效 / 废止"事件。
          </SheetDescription>
        </SheetHeader>
        <div className='p-4'>
          {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
          {!isLoading && data.length === 0 && (
            <p className='text-sm text-muted-foreground'>暂无历史记录。</p>
          )}
          <ol className='relative border-l border-muted-foreground/30 pl-4'>
            {data.map((row) => (
              <li key={row.id} className='mb-6 ms-2'>
                <span className='absolute -start-1.5 mt-1.5 inline-block h-3 w-3 rounded-full bg-primary' />
                <div className='flex items-center gap-2'>
                  <span className='font-medium'>{formatCNY(row.amount_cny_cents)}</span>
                  {!row.effective_to && <Badge>当前生效</Badge>}
                </div>
                <p className='text-sm text-muted-foreground'>
                  生效: {row.effective_from}
                  {row.effective_to && ` → 废止: ${row.effective_to}`}
                </p>
                {row.notes && <p className='text-xs mt-1'>{row.notes}</p>}
              </li>
            ))}
          </ol>
        </div>
      </SheetContent>
    </Sheet>
  )
}
```

- [ ] **Step 3: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/features/pricing/components/pricing-entry-drawer.tsx apps/web/src/features/pricing/components/pricing-history-sheet.tsx
git commit -m "$(cat <<'EOF'
feat(web): pricing entry drawer + history sheet

Drawer: admins input amount in 元 with one-to-two decimals, form
converts to cents on submit. Saving calls the create-or-replace
endpoint (which auto-deprecates the previous active version server-
side). A dedicated "废止当前版本" button calls the deprecate endpoint
directly when admin wants to remove without replacement. History sheet
renders the versioned timeline with a "当前生效" badge on the active
row.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 页面组件 + 路由（含 beforeLoad 门卫）

**Files:**
- Create: `apps/web/src/routes/_authenticated/pricing/index.tsx`
- Create: `apps/web/src/features/pricing/index.tsx`

- [ ] **Step 1: 路由**

```tsx
// apps/web/src/routes/_authenticated/pricing/index.tsx
import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { meQueryOptions } from '@/features/auth/hooks'
import { countriesQueryOptions } from '@/features/catalog/hooks'
import { pricingListQueryOptions } from '@/features/pricing/hooks'
import { Pricing } from '@/features/pricing'

const searchSchema = z.object({
  country_id: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/pricing/')({
  validateSearch: searchSchema,
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'reviewer' && user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loaderDeps: ({ search }) => ({ search }),
  loader: async ({ context, deps }) => {
    // Countries always; pricing only when country is picked.
    await context.queryClient.ensureQueryData(countriesQueryOptions())
    if (deps.search.country_id) {
      await context.queryClient.ensureQueryData(
        pricingListQueryOptions({ country_id: deps.search.country_id })
      )
    }
  },
  component: Pricing,
})
```

- [ ] **Step 2: Pricing 页面**

```tsx
// apps/web/src/features/pricing/index.tsx
import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useMe } from '@/features/auth/hooks'
import { useCountries } from '@/features/catalog/hooks'
import { usePricingList } from './hooks'
import { PricingMatrix } from './components/pricing-matrix'
import { PricingEntryDrawer } from './components/pricing-entry-drawer'
import { PricingHistorySheet } from './components/pricing-history-sheet'
import type { PricingEntry, ServiceTier } from './types'

const route = getRouteApi('/_authenticated/pricing/')

export function Pricing() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const me = useMe()
  const canEdit = me.data?.role === 'admin'

  const { data: countries = [] } = useCountries(false)
  const selected = search.country_id ?? countries[0]?.id ?? ''
  const { data: entries = [], isLoading } = usePricingList({ country_id: selected })

  const [editing, setEditing] = useState<{ feeItem: string; tier: ServiceTier; current?: PricingEntry } | null>(null)
  const [history, setHistory] = useState<{ feeItem: string; tier: ServiceTier } | null>(null)
  const [newFeeItem, setNewFeeItem] = useState('')

  const setCountry = (id: string) => navigate({ search: (s) => ({ ...s, country_id: id }), replace: false })

  const startNewEntry = () => {
    const fee = newFeeItem.trim()
    if (!fee) return
    // Default to basic tier when creating a brand-new fee_item row.
    setEditing({ feeItem: fee, tier: 'basic', current: undefined })
    setNewFeeItem('')
  }

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>定价管理</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>定价管理</h2>
            <p className='text-muted-foreground'>
              按国家 × 服务级别 × 费用项维护报价模板。管理员可编辑；审核员只读。
            </p>
          </div>
          <div className='flex items-center gap-2'>
            <Select value={selected} onValueChange={setCountry}>
              <SelectTrigger className='w-56'>
                <SelectValue placeholder='选择国家' />
              </SelectTrigger>
              <SelectContent>
                {countries.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name_zh} · {c.code}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {canEdit && selected && (
          <div className='flex items-center gap-2'>
            <input
              className='rounded-md border px-3 py-1.5 text-sm'
              placeholder='新费用项名称（如 application_fee）'
              value={newFeeItem}
              onChange={(e) => setNewFeeItem(e.target.value)}
            />
            <Button size='sm' onClick={startNewEntry} disabled={!newFeeItem.trim()}>
              新增条目
            </Button>
          </div>
        )}

        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}

        {selected && (
          <PricingMatrix
            entries={entries}
            canEdit={canEdit}
            onEditCell={(feeItem, tier, current) => setEditing({ feeItem, tier, current })}
            onOpenHistory={(feeItem, tier) => setHistory({ feeItem, tier })}
          />
        )}
      </Main>

      {editing && selected && (
        <PricingEntryDrawer
          open={!!editing}
          onOpenChange={(o) => !o && setEditing(null)}
          countryId={selected}
          feeItem={editing.feeItem}
          serviceTier={editing.tier}
          current={editing.current}
        />
      )}

      {history && selected && (
        <PricingHistorySheet
          open={!!history}
          onOpenChange={(o) => !o && setHistory(null)}
          countryId={selected}
          feeItem={history.feeItem}
          serviceTier={history.tier}
        />
      )}
    </>
  )
}
```

- [ ] **Step 3: Select primitive check**

```bash
ls apps/web/src/components/ui/select.tsx 2>/dev/null || pnpm -C apps/web dlx shadcn@latest add select
```

若已存在就跳过。

- [ ] **Step 4: build + 完整测试**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
pnpm -C apps/web test --run
```

Expected: build 绿；全部已有测试仍绿。

- [ ] **Step 5: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/routes/_authenticated/pricing/ apps/web/src/features/pricing/index.tsx
# 若 shadcn 生成了 select.tsx:
git add apps/web/src/components/ui/select.tsx 2>/dev/null || true
# vite 可能会重建 routeTree:
git add apps/web/src/routeTree.gen.ts
git commit -m "$(cat <<'EOF'
feat(web): pricing page with country picker + matrix + drawer + history

Route beforeLoad reads cached /auth/me and redirects non-{reviewer,admin}
to /403. URL search param country_id selects which country to view;
defaults to first returned by useCountries. Admin sees "新增条目" row
to bootstrap a never-priced fee_item. PricingMatrix dispatches cell
clicks to PricingEntryDrawer (edit) and PricingHistorySheet (read-only
timeline).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: MSW handlers + 集成测试

**Files:**
- Modify: `apps/web/src/test-utils/msw/handlers.ts`
- Create: `apps/web/src/features/pricing/pricing.integration.test.tsx`

- [ ] **Step 1: 扩展 handlers**

打开 `apps/web/src/test-utils/msw/handlers.ts`。加一个 in-memory 定价存储 + 三个路由：

在 `let customers: Array<...> = []` 下面加：

```ts
// In-memory pricing entries keyed by id.
let pricingEntries: Array<{
  id: string
  country_id: string
  service_tier: 'basic' | 'standard' | 'premium'
  fee_item: string
  amount_cny_cents: number
  notes: string | null
  effective_from: string
  effective_to: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []
```

在 `resetMswState` 里加一行 `pricingEntries = []`。

在 `defaultHandlers` 里，在最后 `// ---- catalog minimal handlers ...` 之前插入：

```ts
  // ---- pricing ----
  http.get('/api/v1/pricing-entries', ({ request }) => {
    const url = new URL(request.url)
    const country = url.searchParams.get('country_id')
    const tier = url.searchParams.get('service_tier')
    const items = pricingEntries.filter(
      (p) =>
        p.effective_to == null &&
        (!country || p.country_id === country) &&
        (!tier || p.service_tier === tier)
    )
    return HttpResponse.json({ items })
  }),
  http.get('/api/v1/pricing-entries/history', ({ request }) => {
    const url = new URL(request.url)
    const country = url.searchParams.get('country_id') ?? ''
    const tier = url.searchParams.get('service_tier') ?? ''
    const fee = url.searchParams.get('fee_item') ?? ''
    const items = pricingEntries
      .filter((p) => p.country_id === country && p.service_tier === tier && p.fee_item === fee)
      .slice()
      .sort((a, b) => (a.effective_from < b.effective_from ? 1 : -1))
    return HttpResponse.json({ items })
  }),
  http.post('/api/v1/pricing-entries', async ({ request }) => {
    const body = (await request.json()) as {
      country_id: string
      service_tier: 'basic' | 'standard' | 'premium'
      fee_item: string
      amount_cny_cents: number
      notes?: string | null
      effective_from: string
    }
    // Deprecate existing active for dimension.
    for (const p of pricingEntries) {
      if (
        p.country_id === body.country_id &&
        p.service_tier === body.service_tier &&
        p.fee_item === body.fee_item &&
        p.effective_to == null
      ) {
        p.effective_to = body.effective_from
      }
    }
    const now = new Date().toISOString()
    const row = {
      id: 'p_' + Math.random().toString(36).slice(2, 10),
      country_id: body.country_id,
      service_tier: body.service_tier,
      fee_item: body.fee_item,
      amount_cny_cents: body.amount_cny_cents,
      notes: body.notes ?? null,
      effective_from: body.effective_from,
      effective_to: null,
      created_by: '00000000-0000-0000-0000-000000000001',
      created_at: now,
      updated_at: now,
    }
    pricingEntries.push(row)
    return HttpResponse.json(row, { status: 201 })
  }),
  http.post('/api/v1/pricing-entries/:id/deprecate', async ({ params, request }) => {
    const body = (await request.json().catch(() => ({}))) as { effective_to?: string }
    const row = pricingEntries.find((p) => p.id === params.id)
    if (!row) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (row.effective_to) {
      return HttpResponse.json(
        { code: 'ERR_ALREADY_DEPRECATED', message: 'already deprecated' },
        { status: 409 }
      )
    }
    row.effective_to = body.effective_to ?? new Date(Date.now() + 86_400_000).toISOString().slice(0, 10)
    return HttpResponse.json(row)
  }),
```

还需要让 /catalog/countries 返回一条 country 以便测试挑选：把最后那个 `http.get('/api/v1/catalog/countries', () => HttpResponse.json({ items: [] }))` 改成返回一条假数据：

```ts
  http.get('/api/v1/catalog/countries', () => {
    return HttpResponse.json({
      items: [
        {
          id: 'c_cn_01',
          code: 'CN',
          name_zh: '中国',
          name_en: 'China',
          is_madrid_member: true,
          requires_notarization: false,
          sort_order: 0,
          enabled: true,
        },
      ],
    })
  }),
```

(Plan 5 的客户测试不 care countries handler 的返回，不 break。)

- [ ] **Step 2: 集成测试**

```tsx
// apps/web/src/features/pricing/pricing.integration.test.tsx
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
} from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { SidebarProvider } from '@/components/ui/sidebar'
import { worker } from '@/test-utils/msw/server'
import { resetMswState } from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { Pricing } from '@/features/pricing'

// Pre-hydrate the /auth/me cache so beforeLoad gate passes as admin.
function buildAdminRouter() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  queryClient.setQueryData(['auth', 'me'], {
    id: '00000000-0000-0000-0000-000000000001',
    name: 'Bootstrap Admin',
    email: 'admin@example.com',
    phone: '',
    role: 'admin',
    status: 'active',
  })
  useAuthStore.getState().auth.setUser({
    id: '00000000-0000-0000-0000-000000000001',
    name: 'Bootstrap Admin',
    email: 'admin@example.com',
    phone: '',
    role: 'admin',
    status: 'active',
  })

  const rootRoute = createRootRoute({
    component: () => (
      <SidebarProvider>
        <Outlet />
        <Toaster />
      </SidebarProvider>
    ),
  })
  const pricingRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/pricing',
    validateSearch: (s: Record<string, unknown>) => ({
      country_id: (s.country_id as string | undefined) ?? undefined,
    }),
    component: Pricing,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([pricingRoute]),
    history: createMemoryHistory({ initialEntries: ['/pricing'] }),
    context: { queryClient },
  })
  return { router, queryClient }
}

describe('pricing integration', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'bypass' })
  })
  beforeEach(() => {
    resetMswState()
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
  })
  afterAll(() => {
    worker.stop()
  })

  it('admin can create a new pricing entry and see it in the matrix', async () => {
    const { router, queryClient } = buildAdminRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Country picker defaults to CN (the only one returned by MSW).
    await expect.element(screen.getByText('定价管理')).toBeInTheDocument()

    // Empty state prompts admin.
    await expect.element(screen.getByText(/暂无定价条目/)).toBeInTheDocument()

    // Enter new fee_item name + click 新增条目
    await userEvent.fill(screen.getByPlaceholder(/新费用项名称/), 'application_fee')
    await userEvent.click(screen.getByRole('button', { name: '新增条目' }))

    // Drawer opens with 新增 title
    await expect.element(screen.getByText(/新增定价.*application_fee/)).toBeInTheDocument()

    // Fill amount + save
    await userEvent.fill(screen.getByLabelText('金额（人民币元）'), '800')
    await userEvent.click(screen.getByRole('button', { name: '保存' }))

    // Matrix now shows the row with the amount formatted
    await expect.element(screen.getByText('¥800.00')).toBeInTheDocument()
    await expect.element(screen.getByText('application_fee')).toBeInTheDocument()
  })
})
```

- [ ] **Step 3: 跑集成测试**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web test --run src/features/pricing/pricing.integration.test.tsx
```

Expected: 1 PASS。

- [ ] **Step 4: 全量 build + test + lint**

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web build
pnpm -C apps/web test --run
pnpm -C apps/web lint
```

Expected: all green。

- [ ] **Step 5: commit**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add apps/web/src/test-utils/msw/handlers.ts apps/web/src/features/pricing/pricing.integration.test.tsx
git commit -m "$(cat <<'EOF'
test(web): pricing integration test + MSW pricing stubs

Handlers grow in-memory pricingEntries with the full CRUD shape plus a
single CN country stub for /catalog/countries so the picker resolves.
Integration test walks admin → /pricing → 新增条目 → drawer → save →
matrix shows formatted ¥800.00 row.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Plan 7 Definition of Done

1. ✅ `pnpm -C apps/web build` + `test --run` + `lint` 全绿
2. ✅ /pricing 页面对 salesperson 跳 /403；reviewer 只读；admin 可编辑
3. ✅ Sidebar 新增 "业务 > 定价" 对 reviewer/admin 可见
4. ✅ 新增 fee_item + 保存 → 矩阵出现金额
5. ✅ 点"历史" → 时间线出现该维度所有版本
6. ✅ 点"修改"保存 → 旧版本 effective_to 被设，新版本加到历史
7. ✅ 点"废止当前版本" → 该维度矩阵格变空

## 下一步

Plan 8：后端报价 + 工作流 — quotation 表（含 quotation_items 子表）+ 状态机 draft/submitted/approved/rejected + 提交/审核/拒绝 transitions + audit 集成 + 调用 pricing.Calculate 快照定价签名。
