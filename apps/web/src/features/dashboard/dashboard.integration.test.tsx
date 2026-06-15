import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { render } from 'vitest-browser-react'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
  createRootRoute,
  createRoute,
  Outlet,
} from '@tanstack/react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SidebarProvider } from '@/components/ui/sidebar'
import { worker } from '@/test-utils/msw/server'
import { resetMswState, seedCustomer, seedQuotationDraft } from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { Dashboard } from '@/features/dashboard'

function buildRouter() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useAuthStore.getState().auth.setUser({
    id: '00000000-0000-0000-0000-000000000001',
    name: 'Admin',
    email: 'admin@example.com',
    phone: '',
    role: 'admin',
    status: 'active',
  })
  const rootRoute = createRootRoute({
    component: () => (
      <SidebarProvider>
        <Outlet />
      </SidebarProvider>
    ),
  })
  const dashRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: Dashboard,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([dashRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
    context: { queryClient },
  })
  return { router, queryClient }
}

describe('dashboard integration', () => {
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

  it('renders KPIs and the recent quotations list', async () => {
    // Seed a customer + two quotations.
    const custId = seedCustomer({ name: 'Test Customer' })
    seedQuotationDraft({
      customer_id: custId,
      country_id: '00000000-0000-0000-0000-000000000100',
      service_tier: 'basic',
    })
    seedQuotationDraft({
      customer_id: custId,
      country_id: '00000000-0000-0000-0000-000000000100',
      service_tier: 'standard',
    })

    const { router, queryClient } = buildRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Heading visible.
    await expect.element(screen.getByRole('heading', { name: '仪表盘' })).toBeInTheDocument()

    // With 2 draft quotations seeded, "全公司报价总数" card shows 2.
    await expect.element(screen.getByText('全公司报价总数')).toBeInTheDocument()
    await expect.element(screen.getByText('2').first()).toBeInTheDocument()

    // Recent quotations heading shows.
    await expect.element(screen.getByText('状态分布')).toBeInTheDocument()
    await expect.element(screen.getByText('近期报价')).toBeInTheDocument()
    await expect.element(screen.getByText('基础')).toBeInTheDocument()
    await expect.element(screen.getByText('标准')).toBeInTheDocument()
    await expect.element(screen.getByText('basic')).not.toBeInTheDocument()
    await expect.element(screen.getByText('standard')).not.toBeInTheDocument()
  })
})
