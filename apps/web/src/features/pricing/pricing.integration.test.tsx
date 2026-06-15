import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
  createRootRoute,
  createRoute,
  Outlet,
} from '@tanstack/react-router'
import { resetMswState } from '@/test-utils/msw/handlers'
import { worker } from '@/test-utils/msw/server'
import { Toaster } from 'sonner'
import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { SidebarProvider } from '@/components/ui/sidebar'
import { Pricing } from '@/features/pricing'

// Pre-hydrate the /auth/me cache so beforeLoad gate passes as admin.
function buildAdminRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const admin = {
    id: '00000000-0000-0000-0000-000000000001',
    name: 'Bootstrap Admin',
    email: 'admin@example.com',
    phone: '',
    role: 'admin' as const,
    status: 'active' as const,
  }
  queryClient.setQueryData(['auth', 'me'], admin)
  useAuthStore.getState().auth.setUser(admin)

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

  it('admin can create single class pricing and see it in the table', async () => {
    const { router, queryClient } = buildAdminRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await expect
      .element(screen.getByRole('heading', { name: '定价管理' }))
      .toBeInTheDocument()

    await expect
      .element(screen.getByText(/暂无单一分类定价/))
      .toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: '新增单一分类定价' })
    )

    const dialog = screen.getByRole('dialog')
    await expect
      .element(dialog.getByRole('heading', { name: '单一分类定价', level: 2 }))
      .toBeInTheDocument()

    const countrySelect = dialog.getByRole('combobox', { name: '国家/地区' })
    await userEvent.click(countrySelect)
    await expect
      .element(screen.getByRole('option', { name: '阿根廷 · AR' }))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('option', { name: '阿根廷 · AR' }))

    await userEvent.fill(dialog.getByLabelText('大洲'), '南美洲')
    await userEvent.fill(dialog.getByLabelText('首类费用（不含税）'), '3600')
    await expect
      .element(dialog.getByLabelText('首类费用（含税 6%）'))
      .toHaveValue('3816.00')
    await expect
      .element(dialog.getByLabelText('首类费用（含税 1%）'))
      .toHaveValue('3636.00')
    await userEvent.fill(dialog.getByLabelText('每次类费用（不含税）'), '2700')
    await expect
      .element(dialog.getByLabelText('每次类费用（含税 6%）'))
      .toHaveValue('2862.00')
    await expect
      .element(dialog.getByLabelText('每次类费用（含税 1%）'))
      .toHaveValue('2727.00')
    await userEvent.fill(dialog.getByLabelText('受理需时'), '2 days')
    await userEvent.fill(dialog.getByLabelText('注册需时（月）'), '6--8')
    await userEvent.click(dialog.getByRole('button', { name: '保存' }))

    const table = screen.getByRole('table')
    await expect.element(table.getByText('阿根廷')).toBeInTheDocument()
    await expect.element(table.getByText('¥3,600.00')).toBeInTheDocument()
    await expect.element(table.getByText('¥3,816.00')).toBeInTheDocument()
    await expect.element(table.getByText('¥3,636.00')).toBeInTheDocument()
  })

  it('madrid base pricing drawer does not ask for a country or region', async () => {
    const { router, queryClient } = buildAdminRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

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
    const { router, queryClient } = buildAdminRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await userEvent.click(screen.getByRole('tab', { name: '马德里' }))
    await userEvent.click(screen.getByRole('button', { name: '新增国家费' }))

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
      .element(screen.getByRole('option', { name: '美国 · US' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('阿根廷 · AR'))
      .not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('option', { name: '美国 · US' }))
    await userEvent.fill(dialog.getByLabelText('官费（瑞士法郎）'), '261')
    await userEvent.fill(dialog.getByLabelText('我所代理费（人民币元）'), '400')
    await userEvent.click(dialog.getByRole('button', { name: '保存' }))

    const table = screen.getByRole('table')
    await expect.element(table.getByText('美国')).toBeInTheDocument()
    await expect.element(table.getByText('CHF 261.00')).toBeInTheDocument()
  })
})
