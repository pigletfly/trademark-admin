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

  it('admin can create a new pricing entry and see it in the matrix', async () => {
    const { router, queryClient } = buildAdminRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await expect.element(screen.getByRole('heading', { name: '定价管理' })).toBeInTheDocument()

    // Empty state prompt when no entries exist yet.
    await expect.element(screen.getByText(/暂无定价条目/)).toBeInTheDocument()

    // Enter new fee_item name + click 新增条目
    await userEvent.fill(screen.getByPlaceholder(/新费用项名称/), 'application_fee')
    await userEvent.click(screen.getByRole('button', { name: '新增条目' }))

    // Drawer opens with 新增 title
    await expect.element(screen.getByText(/新增定价.*application_fee/)).toBeInTheDocument()

    // Fill amount + save
    await userEvent.fill(screen.getByLabelText('金额（人民币元）'), '800')
    await userEvent.click(screen.getByRole('button', { name: '保存' }))

    // Matrix renders the new row — scope to the table so we don't accept
    // the string from the still-mounted drawer or toast.
    const matrix = screen.getByRole('table')
    await expect.element(matrix.getByText('application_fee')).toBeInTheDocument()
    await expect.element(matrix.getByText('¥800.00')).toBeInTheDocument()
  })
})
