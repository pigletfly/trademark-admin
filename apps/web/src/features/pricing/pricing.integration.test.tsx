import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
  createRootRoute,
  createRoute,
  Outlet,
} from '@tanstack/react-router'
import {
  resetMswState,
  seedMadridPricingEntry,
  seedSingleClassPricingEntry,
} from '@/test-utils/msw/handlers'
import { worker } from '@/test-utils/msw/server'
import { Toaster } from 'sonner'
import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { SidebarProvider } from '@/components/ui/sidebar'
import { Pricing } from '@/features/pricing'

const COUNTRY_IDS = {
  CN: '00000000-0000-0000-0000-000000000100',
  US: '00000000-0000-0000-0000-000000000101',
  AR: '00000000-0000-0000-0000-000000000102',
} as const

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
    role: args.role ?? 'admin',
    status: 'active' as const,
  }
  queryClient.setQueryData(['auth', 'me'], user)
  useAuthStore.getState().auth.setUser(user)

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
    history: createMemoryHistory({
      initialEntries: args.initialEntries ?? ['/pricing'],
    }),
    context: { queryClient },
  })
  return { router, queryClient }
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

    await expect
      .element(screen.getByRole('heading', { name: '定价管理' }))
      .toBeInTheDocument()

    await expect
      .element(screen.getByRole('table').getByText('美国'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('table').getByText('阿根廷'))
      .toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('combobox', { name: '单一分类国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )

    await expect
      .element(screen.getByRole('table').getByText('美国'))
      .toBeInTheDocument()
    await expect.element(screen.getByText('阿根廷')).not.toBeInTheDocument()
  })

  it('single-class create and edit flows follow filter-scoped rules', async () => {
    seedSingleClassPricingEntry({
      country_id: COUNTRY_IDS.US,
      continent: '北美洲',
      country_area: '美国',
      first_class_fee_cny_cents: 790000,
      additional_class_fee_cny_cents: 20000,
    })

    const screen = await renderPricing()

    await userEvent.click(
      screen.getByRole('combobox', { name: '单一分类国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )

    const addButton = screen.getByRole('button', { name: '新增单一分类定价' })
    await expect.element(addButton).toBeDisabled()
    await expect
      .element(screen.getByText('当前筛选国家已有生效定价，请使用行内修改。'))
      .toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: '修改' }))

    const editDialog = screen.getByRole('dialog')
    await expect
      .element(
        editDialog.getByRole('heading', { name: '单一分类定价', level: 2 })
      )
      .toBeInTheDocument()
    await expect
      .element(editDialog.getByRole('combobox', { name: '国家/地区' }))
      .not.toBeInTheDocument()
    await userEvent.click(editDialog.getByRole('button', { name: '取消' }))

    await userEvent.click(
      screen.getByRole('combobox', { name: '单一分类国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:阿根廷|Argentina) · AR/ })
    )

    await expect
      .element(screen.getByRole('button', { name: '新增单一分类定价' }))
      .toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: '新增单一分类定价' }))

    const createDialog = screen.getByRole('dialog')
    await expect
      .element(createDialog.getByText(/阿根廷|Argentina/))
      .toBeInTheDocument()
    await expect
      .element(createDialog.getByRole('combobox', { name: '国家/地区' }))
      .not.toBeInTheDocument()
  })

  it('madrid tab splits base and country pricing while keeping base visible under filters', async () => {
    seedMadridPricingEntry({
      country_area: 'Basic registration fee - black and white mark',
      official_fee_chf_cents: 65300,
      agency_fee_cny_cents: 120000,
      is_base_fee: true,
    })
    seedMadridPricingEntry({
      country_id: COUNTRY_IDS.US,
      sequence_no: 1,
      country_area: 'United States',
      official_fee_chf_cents: 26100,
      agency_fee_cny_cents: 40000,
      is_base_fee: false,
    })

    const screen = await renderPricing()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))

    await expect
      .element(screen.getByRole('heading', { name: '马德里基础费' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('heading', { name: '马德里国家费' }))
      .toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('combobox', { name: '马德里国家费国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )

    const baseTable = screen.getByRole('table', { name: '马德里基础费表格' })
    const countryTable = screen.getByRole('table', { name: '马德里国家费表格' })

    await expect.element(baseTable.getByText('基础注册费')).toBeInTheDocument()
    await expect
      .element(countryTable.getByRole('button', { name: '修改' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '修改基础费' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '修改国家费' }))
      .not.toBeInTheDocument()
  })

  it('madrid create flow locks the selected country when filtered to a country without pricing', async () => {
    seedMadridPricingEntry({
      country_area: 'Basic registration fee - black and white mark',
      official_fee_chf_cents: 65300,
      agency_fee_cny_cents: 120000,
      is_base_fee: true,
    })

    const screen = await renderPricing()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))
    await userEvent.click(
      screen.getByRole('combobox', { name: '马德里国家费国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )

    await expect
      .element(screen.getByText('当前筛选国家暂无生效定价。'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '新增马德里国家费' }))
      .toBeEnabled()

    await userEvent.click(
      screen.getByRole('button', { name: '新增马德里国家费' })
    )

    const dialog = screen.getByRole('dialog')
    await expect
      .element(dialog.getByRole('heading', { name: '马德里国家费', level: 2 }))
      .toBeInTheDocument()
    await expect
      .element(dialog.getByText(/美国|United States/))
      .toBeInTheDocument()
    await expect
      .element(dialog.getByRole('combobox', { name: '国家/地区' }))
      .not.toBeInTheDocument()
  })

  it('madrid base section shows an empty state and create button when base pricing is missing', async () => {
    const screen = await renderPricing()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))

    await expect.element(screen.getByText('暂无基础费。')).toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '新增基础费' }))
      .toBeInTheDocument()
  })

  it('madrid base pricing drawer does not ask for a country or region', async () => {
    const screen = await renderPricing()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))
    await userEvent.click(screen.getByRole('button', { name: '新增基础费' }))

    await expect
      .element(screen.getByRole('heading', { name: '马德里基础费', level: 2 }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByLabelText('国家/地区'))
      .not.toBeInTheDocument()
  })

  it('madrid country pricing drawer uses a Madrid-member country dropdown', async () => {
    const screen = await renderPricing()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))
    await userEvent.click(
      screen.getByRole('button', { name: '新增马德里国家费' })
    )

    const dialog = screen.getByRole('dialog')
    await expect
      .element(dialog.getByRole('heading', { name: '马德里国家费', level: 2 }))
      .toBeInTheDocument()

    const countrySelect = dialog.getByRole('combobox', { name: '国家/地区' })
    await userEvent.click(countrySelect)

    await expect
      .element(screen.getByRole('option', { name: '中国 · CN' }))
      .toBeInTheDocument()
    await expect
      .element(
        screen.getByRole('option', { name: /(?:美国|United States) · US/ })
      )
      .toBeInTheDocument()
    await expect
      .element(screen.getByText(/(?:阿根廷|Argentina) · AR/))
      .not.toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )
    await userEvent.fill(dialog.getByLabelText('官费（瑞士法郎）'), '261')
    await userEvent.fill(dialog.getByLabelText('我所代理费（人民币元）'), '400')
    await userEvent.click(dialog.getByRole('button', { name: '保存' }))

    const table = screen.getByRole('table')
    await expect.element(table.getByText(/美国|United States/)).toBeInTheDocument()
    await expect.element(table.getByText('CHF 261.00')).toBeInTheDocument()
  })

  it('reviewer sees pricing tables without create or edit actions', async () => {
    seedSingleClassPricingEntry({
      country_id: COUNTRY_IDS.US,
      continent: '北美洲',
      country_area: '美国',
      first_class_fee_cny_cents: 790000,
      additional_class_fee_cny_cents: 20000,
    })
    seedMadridPricingEntry({
      country_area: 'Basic registration fee - black and white mark',
      official_fee_chf_cents: 65300,
      agency_fee_cny_cents: 120000,
      is_base_fee: true,
    })
    seedMadridPricingEntry({
      country_id: COUNTRY_IDS.US,
      sequence_no: 1,
      country_area: 'United States',
      official_fee_chf_cents: 26100,
      agency_fee_cny_cents: 40000,
      is_base_fee: false,
    })

    const screen = await renderPricing({ role: 'reviewer' })

    await expect
      .element(screen.getByRole('table').getByText('美国'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '新增单一分类定价' }))
      .not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))

    await expect
      .element(screen.getByRole('table', { name: '马德里基础费表格' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('table', { name: '马德里国家费表格' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '新增基础费' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '新增马德里国家费' }))
      .not.toBeInTheDocument()
    await expect.element(screen.getByText('操作')).not.toBeInTheDocument()
  })

  it('shared country filter carries from single-class to madrid country fees', async () => {
    seedSingleClassPricingEntry({
      country_id: COUNTRY_IDS.US,
      continent: '北美洲',
      country_area: '美国',
      first_class_fee_cny_cents: 790000,
      additional_class_fee_cny_cents: 20000,
    })
    seedMadridPricingEntry({
      country_area: 'Basic registration fee - black and white mark',
      official_fee_chf_cents: 65300,
      agency_fee_cny_cents: 120000,
      is_base_fee: true,
    })
    seedMadridPricingEntry({
      country_id: COUNTRY_IDS.US,
      sequence_no: 1,
      country_area: 'United States',
      official_fee_chf_cents: 26100,
      agency_fee_cny_cents: 40000,
      is_base_fee: false,
    })

    const screen = await renderPricing()

    await userEvent.click(
      screen.getByRole('combobox', { name: '单一分类国家筛选' })
    )
    await userEvent.click(
      screen.getByRole('option', { name: /(?:美国|United States) · US/ })
    )

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))

    const madridFilter = screen.getByRole('combobox', {
      name: '马德里国家费国家筛选',
    })
    const baseTable = screen.getByRole('table', { name: '马德里基础费表格' })
    const countryTable = screen.getByRole('table', { name: '马德里国家费表格' })

    await expect
      .element(madridFilter)
      .toHaveTextContent(/(?:美国|United States) · US/)
    await expect.element(baseTable.getByText('基础注册费')).toBeInTheDocument()
    await expect
      .element(countryTable.getByText(/美国|United States/))
      .toBeInTheDocument()
  })
})
