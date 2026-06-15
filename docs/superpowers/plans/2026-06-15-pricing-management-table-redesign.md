# Pricing Management Table Redesign Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework `/pricing` into table-first single-class and Madrid pricing views with in-table country filters, default all-country listing, row-level edit actions, and create/edit drawer modes that preserve the existing backend API.

**Architecture:** Keep the runtime surface area tight: reuse the existing `Pricing` page, the existing method-pricing drawers, and the existing TanStack Query hooks. Put most behavior changes into page-level derived state in `index.tsx`, add explicit `create` / `edit` drawer modes in `method-pricing-drawers.tsx`, and cover the new behavior with integration tests that seed MSW pricing rows directly instead of inventing new backend stubs.

**Tech Stack:** React 19 + TanStack Router + TanStack Query + react-hook-form + zod + shadcn/ui + Vitest Browser + MSW. No new dependencies.

---

## File Structure

### Modify

- `apps/web/src/features/pricing/index.tsx`
  - Replace the page-level country selector with per-table selectors.
  - Treat missing `country_id` as “全部国家/地区”.
  - Derive single-class rows, Madrid base row, Madrid country rows, disabled reasons, and row-level edit actions from the fetched data.
- `apps/web/src/features/pricing/components/method-pricing-drawers.tsx`
  - Introduce explicit `create` / `edit` behavior for both single-class and Madrid country drawers.
  - Make country selection read-only in edit mode and optionally locked in create mode when the current filter preselects a single country.
- `apps/web/src/routes/_authenticated/pricing/index.tsx`
  - Align route prefetching with the actual page data: country list + single-class pricing list + Madrid pricing list.
- `apps/web/src/features/pricing/pricing.integration.test.tsx`
  - Replace the current minimal create-only assertions with seeded all-country, filter, disabled-action, drawer-mode, and reviewer-permission scenarios.

### Reuse as-is

- `apps/web/src/features/pricing/hooks/use-method-pricing.ts`
  - Already supports optional `country_id`; no contract changes needed.
- `apps/web/src/test-utils/msw/handlers.ts`
  - Already exports `seedSingleClassPricingEntry` and `seedMadridPricingEntry`; tests should use those helpers rather than editing handler behavior.

### Test Data Constants

Use the static country IDs already exposed by the catalog MSW handler:

```ts
const COUNTRY_IDS = {
  CN: '00000000-0000-0000-0000-000000000100',
  US: '00000000-0000-0000-0000-000000000101',
  AR: '00000000-0000-0000-0000-000000000102',
} as const
```

These IDs keep the tests deterministic and avoid coupling the plan to DOM text matching alone.

---

## Task 1: Single-class tab becomes a default-all table with filter-scoped actions

**Files:**
- Modify: `apps/web/src/features/pricing/pricing.integration.test.tsx`
- Modify: `apps/web/src/features/pricing/index.tsx`
- Modify: `apps/web/src/features/pricing/components/method-pricing-drawers.tsx`

- [ ] **Step 1: Write the failing single-class integration tests**

Add imports for the MSW seed helpers and replace the current single create-only test with two seeded single-class tests:

```ts
import {
  resetMswState,
  seedSingleClassPricingEntry,
} from '@/test-utils/msw/handlers'

const COUNTRY_IDS = {
  CN: '00000000-0000-0000-0000-000000000100',
  US: '00000000-0000-0000-0000-000000000101',
  AR: '00000000-0000-0000-0000-000000000102',
} as const
```

Replace the current one-off `buildAdminRouter()` helper with a reusable pair:

```ts
function buildPricingRouter(args: {
  role?: 'admin' | 'reviewer'
  initialEntries?: string[]
} = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const user = {
    id: '00000000-0000-0000-0000-000000000001',
    name: args.role === 'reviewer' ? 'Bootstrap Reviewer' : 'Bootstrap Admin',
    email: 'user@example.com',
    phone: '',
    role: (args.role ?? 'admin') as const,
    status: 'active' as const,
  }
  queryClient.setQueryData(['auth', 'me'], user)
  useAuthStore.getState().auth.setUser(user)
  // reuse the same rootRoute/pricingRoute structure, but feed initialEntries
}

async function renderPricing(args?: {
  role?: 'admin' | 'reviewer'
  initialEntries?: string[]
}) {
  const { router, queryClient } = buildPricingRouter(args)
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
```

Test A should seed US + AR single-class pricing and assert:

```ts
it('admin sees all single-class rows by default and can filter to one country', async () => {
  seedSingleClassPricingEntry({
    country_id: COUNTRY_IDS.US,
    continent: '北美洲',
    country_area: '美国',
    first_class_fee_cny_cents: 790000,
    additional_class_fee_cny_cents: 20000,
  })
  seedSingleClassPricingEntry({
    country_id: COUNTRY_IDS.AR,
    continent: '南美洲',
    country_area: '阿根廷',
    first_class_fee_cny_cents: 360000,
    additional_class_fee_cny_cents: 270000,
  })

  const screen = await renderPricing()

  await expect.element(screen.getByRole('table').getByText('美国')).toBeInTheDocument()
  await expect.element(screen.getByRole('table').getByText('阿根廷')).toBeInTheDocument()

  await userEvent.click(screen.getByRole('combobox', { name: '单一分类国家筛选' }))
  await userEvent.click(screen.getByRole('option', { name: '美国 · US' }))

  await expect.element(screen.getByRole('table').getByText('美国')).toBeInTheDocument()
  await expect.element(screen.getByText('阿根廷')).not.toBeInTheDocument()
})
```

Test B should seed only US single-class pricing and assert:

1. Filtering to US disables `新增单一分类定价`.
2. The disabled-state hint text is visible.
3. Clicking row-level `修改` opens the drawer.
4. In edit mode there is no `国家/地区` combobox.
5. Filtering to AR enables `新增单一分类定价`, opening the drawer with the country locked to AR.

- [ ] **Step 2: Run the targeted tests to confirm the current page fails**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx --testNamePattern "single-class|单一分类"
```

Expected: FAIL because the current page still defaults to the first country, has no “全部国家/地区” option, has no row-level `操作` column, and the drawer still renders a mutable country select in edit mode.

- [ ] **Step 3: Implement single-class page state in `index.tsx`**

At the top of `Pricing`, introduce a sentinel and stop falling back to the first country:

```ts
const ALL_COUNTRIES = '__all__'

const selectedCountryId = search.country_id
const selectedCountry = countries.find((country) => country.id === selectedCountryId)
```

Change the query to respect `undefined`:

```ts
const singleClass = useSingleClassPricingList({
  country_id: selectedCountryId,
})
```

Derive the rows and available create targets from the fetched data:

```ts
const singleRows = useMemo(() => singleClass.data ?? [], [singleClass.data])
const existingSingleCountryIds = useMemo(
  () => new Set(singleRows.map((row) => row.country_id)),
  [singleRows]
)

const singleCreateCountries = useMemo(() => {
  if (selectedCountryId) {
    const selectedCountry = countries.find((country) => country.id === selectedCountryId)
    if (!selectedCountry || existingSingleCountryIds.has(selectedCountryId)) return []
    return [selectedCountry]
  }
  return countries.filter((country) => !existingSingleCountryIds.has(country.id))
}, [countries, existingSingleCountryIds, selectedCountryId])
```

Render a per-table filter header:

```tsx
<Select
  value={selectedCountryId ?? ALL_COUNTRIES}
  onValueChange={(value) =>
    navigate({
      search: (old: PricingSearch) => ({
        ...old,
        country_id: value === ALL_COUNTRIES ? undefined : value,
      }),
      replace: false,
    })
  }
>
  <SelectTrigger aria-label='单一分类国家筛选' className='w-56'>
    <SelectValue placeholder='全部国家/地区' />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value={ALL_COUNTRIES}>全部国家/地区</SelectItem>
    {countries.map((country) => (
      <SelectItem key={country.id} value={country.id}>
        {country.name_zh} · {country.code}
      </SelectItem>
    ))}
  </SelectContent>
</Select>
```

Replace the single-row-only rendering with a table over `singleRows`, and add an admin-only `操作` column that calls `setSingleEditor(row)`.

Also add the disabled-state hint under the section header:

```tsx
{canEdit && singleAddDisabledReason && (
  <p className='text-sm text-muted-foreground'>
    {singleAddDisabledReason}
  </p>
)}
```

Use two empty states:

```tsx
!singleClass.isLoading && singleRows.length === 0 && (
  <p className='text-sm text-muted-foreground'>
    {selectedCountryId
      ? '当前筛选国家暂无生效定价。'
      : '暂无单一分类定价。'}
  </p>
)
```

- [ ] **Step 4: Implement explicit single-class drawer modes**

In `method-pricing-drawers.tsx`, change the single-class drawer signature to accept explicit mode and allowed countries:

```ts
type SingleClassDrawerMode = 'create' | 'edit'

export function SingleClassPricingDrawer({
  mode,
  lockedCountryId,
  availableCountries,
  current,
  ...
}: {
  mode: SingleClassDrawerMode
  lockedCountryId?: string
  availableCountries: Country[]
  current?: SingleClassPricingEntry
  ...
})
```

Reset logic should distinguish the three cases:

1. `edit`: use `current.country_id`, show country as read-only text.
2. `create + lockedCountryId`: prefill that country and show it as read-only text.
3. `create + !lockedCountryId`: show a country select sourced from `availableCountries`.

Use this render split instead of always showing the select:

```tsx
{mode === 'edit' || lockedCountryId ? (
  <ReadOnlyField
    label='国家/地区'
    value={`${resolvedCountry?.name_zh ?? values.country_area} · ${resolvedCountry?.code ?? ''}`}
  />
) : (
  <CountrySelectField countries={availableCountries} ... />
)}
```

On submit, stop falling back to the page-level `countryId`; use the form value that was set by create/edit mode:

```ts
country_id: values.country_id,
```

- [ ] **Step 5: Run the single-class tests again**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx --testNamePattern "single-class|单一分类"
```

Expected: PASS for the new single-class tests. No change to Madrid expectations yet.

- [ ] **Step 6: Commit the single-class slice**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add \
  apps/web/src/features/pricing/index.tsx \
  apps/web/src/features/pricing/components/method-pricing-drawers.tsx \
  apps/web/src/features/pricing/pricing.integration.test.tsx
git commit -m "feat(web): redesign single-class pricing table interactions"
```

---

## Task 2: Madrid tab splits base fee and country fee into separate table sections

**Files:**
- Modify: `apps/web/src/features/pricing/pricing.integration.test.tsx`
- Modify: `apps/web/src/features/pricing/index.tsx`
- Modify: `apps/web/src/features/pricing/components/method-pricing-drawers.tsx`

- [ ] **Step 1: Write the failing Madrid integration tests**

Add three Madrid-focused tests.

Test A seeds one base fee plus US country fee and asserts:

1. The `马德里` tab shows a `马德里基础费` section and a `马德里国家费` section.
2. The base row is still visible after filtering the country-fee selector to US.
3. The country-fee table has an `操作` column with row-level `修改`.

Test B seeds only base fee and asserts:

1. Filtering to US leaves the country-fee table empty with `当前筛选国家暂无生效定价。`
2. `新增马德里国家费` is enabled.
3. Opening that drawer for US shows no `国家/地区` combobox because the country is locked to US.

Test C seeds no base fee and asserts:

1. The base section shows `暂无基础费`.
2. The section-level `新增基础费` button is visible.

- [ ] **Step 2: Run the targeted Madrid tests to confirm failure**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx --testNamePattern "Madrid|马德里"
```

Expected: FAIL because the current page still mixes base + country rows into one table, still uses section-top buttons instead of row-level actions, and the drawer still allows mutable country selection for country-fee edits.

- [ ] **Step 3: Rework the Madrid section in `index.tsx`**

Keep the existing query shape but derive two independent view models:

```ts
const madridRows = useMemo(() => madrid.data ?? [], [madrid.data])
const madridBaseRow = useMemo(
  () => madridRows.find((row) => row.is_base_fee),
  [madridRows]
)
const madridCountryRows = useMemo(
  () => madridRows.filter((row) => !row.is_base_fee),
  [madridRows]
)
const existingMadridCountryIds = useMemo(
  () => new Set(madridCountryRows.flatMap((row) => (row.country_id ? [row.country_id] : []))),
  [madridCountryRows]
)
```

Derive create targets using only Madrid-member countries:

```ts
const madridMemberCountries = useMemo(
  () => countries.filter((country) => country.is_madrid_member),
  [countries]
)

const madridCreateCountries = useMemo(() => {
  if (selectedCountryId) {
    const selectedCountry = madridMemberCountries.find((country) => country.id === selectedCountryId)
    if (!selectedCountry || existingMadridCountryIds.has(selectedCountryId)) return []
    return [selectedCountry]
  }
  return madridMemberCountries.filter(
    (country) => !existingMadridCountryIds.has(country.id)
  )
}, [existingMadridCountryIds, madridMemberCountries, selectedCountryId])
```

Render two sections:

1. `MadridBaseTable` for exactly one base row (or a base empty state).
2. `MadridCountryTable` for `madridCountryRows`.

The country-fee header should own the shared filter:

```tsx
<SelectTrigger aria-label='马德里国家费国家筛选' className='w-56'>
```

Country-fee empty states should match the single-class wording:

```tsx
{selectedCountryId
  ? '当前筛选国家暂无生效定价。'
  : '暂无马德里国家费。'}
```

The base section should keep its own empty state:

```tsx
{!madridBaseRow && <p className='text-sm text-muted-foreground'>暂无基础费。</p>}
```

- [ ] **Step 4: Add explicit Madrid drawer modes**

Replace the current implicit `mode: 'base' | 'country'` usage with a second axis for create vs edit:

```ts
type MadridDrawerKind = 'base' | 'country'
type MadridDrawerMode = 'create' | 'edit'
```

At the page level, open the drawer with one of four states:

```ts
type MadridEditor =
  | { kind: 'base'; mode: 'create' }
  | { kind: 'base'; mode: 'edit'; current: MadridPricingEntry }
  | { kind: 'country'; mode: 'create'; lockedCountryId?: string }
  | { kind: 'country'; mode: 'edit'; current: MadridPricingEntry }
```

Inside `method-pricing-drawers.tsx`:

1. Base mode: keep country hidden.
2. Country create:
   - show a select if multiple `availableCountries` exist;
   - otherwise lock the selected country and render read-only text.
3. Country edit: always render read-only country text.

Reuse the same read-only field helper added for the single-class drawer.

- [ ] **Step 5: Run the Madrid tests again**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx --testNamePattern "Madrid|马德里"
```

Expected: PASS for the new Madrid tests. Single-class tests should still pass if re-run.

- [ ] **Step 6: Commit the Madrid slice**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add \
  apps/web/src/features/pricing/index.tsx \
  apps/web/src/features/pricing/components/method-pricing-drawers.tsx \
  apps/web/src/features/pricing/pricing.integration.test.tsx
git commit -m "feat(web): split madrid pricing into base and country tables"
```

---

## Task 3: Align route prefetching, reviewer permissions, and full-page regression

**Files:**
- Modify: `apps/web/src/routes/_authenticated/pricing/index.tsx`
- Modify: `apps/web/src/features/pricing/pricing.integration.test.tsx`
- Modify: `apps/web/src/features/pricing/index.tsx`

- [ ] **Step 1: Write the failing reviewer + shared-filter regression tests**

Add two final tests.

Test A should build a router with a reviewer user and assert:

1. The single-class and Madrid tables still render data.
2. `新增单一分类定价`, `新增马德里国家费`, and `新增基础费` are absent.
3. No `操作` column is rendered in either tab.

Test B should seed US single-class + US Madrid country fee, then assert:

1. Select US from `单一分类国家筛选`.
2. Switch to the `马德里` tab.
3. The `马德里国家费国家筛选` combobox still shows US.
4. The country-fee table stays scoped to US while the base section remains visible.

- [ ] **Step 2: Run the targeted regression tests to confirm failure**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx --testNamePattern "reviewer|共享筛选|shared filter"
```

Expected: FAIL if reviewer still sees admin actions or if the Madrid tab resets the country filter context.

- [ ] **Step 3: Fix the route loader and permission-dependent rendering**

In `apps/web/src/routes/_authenticated/pricing/index.tsx`, replace the stale prefetch import:

```ts
import {
  madridPricingListQueryOptions,
  singleClassPricingListQueryOptions,
} from '@/features/pricing/hooks'
```

Then prefetch the same data the page actually renders:

```ts
loader: async ({ context, deps }) => {
  await context.queryClient.ensureQueryData(countriesQueryOptions())
  await context.queryClient.ensureQueryData(
    singleClassPricingListQueryOptions({
      country_id: deps.search.country_id,
    })
  )
  await context.queryClient.ensureQueryData(
    madridPricingListQueryOptions({
      country_id: deps.search.country_id,
      include_base: true,
    })
  )
}
```

In `index.tsx`, keep the admin-only checks tight:

1. Wrap every `新增` button in `canEdit`.
2. Only render the `操作` header and cells when `canEdit`.
3. Do not render empty `操作` cells for reviewers.

- [ ] **Step 4: Run the full pricing regression and web build**

Run:

```bash
cd /Users/adam/workspace/github/trademark-admin
pnpm -C apps/web exec vitest run src/features/pricing/pricing.integration.test.tsx
pnpm -C apps/web build
```

Expected:

1. The pricing integration file passes in full.
2. The web build succeeds with no type errors from the new drawer props or loader imports.

- [ ] **Step 5: Commit the regression slice**

```bash
cd /Users/adam/workspace/github/trademark-admin
git add \
  apps/web/src/routes/_authenticated/pricing/index.tsx \
  apps/web/src/features/pricing/index.tsx \
  apps/web/src/features/pricing/pricing.integration.test.tsx
git commit -m "feat(web): finish pricing table redesign route and permission flow"
```

---

## Manual Verification Checklist

- [ ] Open `/pricing` as admin and confirm the page defaults to “全部国家/地区” instead of the first country.
- [ ] In the single-class tab, filter to a country with existing pricing and confirm `新增单一分类定价` is disabled while row-level `修改` still works.
- [ ] In the single-class tab, filter to a country without pricing and confirm create mode locks the country.
- [ ] In the Madrid tab, confirm the base section stays visible while the country-fee section responds to the shared country filter.
- [ ] In the Madrid tab, confirm the base empty state shows `新增基础费` only when no base fee exists.
- [ ] As reviewer, confirm there are no create buttons and no `操作` columns in either tab.

---

## Notes for the Implementer

1. Keep the derived-state chain centralized in `index.tsx`; do not scatter “can create?”, “available countries”, and “disabled reason” logic across click handlers.
2. Do not change the API payload shape. The whole redesign relies on existing `create-or-replace` mutations plus stricter UI constraints.
3. Prefer read-only text over disabled selects in edit mode. The tests should assert the combobox is absent, not merely disabled.
