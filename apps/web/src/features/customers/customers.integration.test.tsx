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
import { Toaster } from 'sonner'
import { worker } from '@/test-utils/msw/server'
import { resetMswState } from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { meQueryOptions } from '@/features/auth/hooks'
import { __resetAuthInterceptorState } from '@/lib/api'
import { SignIn } from '@/features/auth/sign-in'
import { Customers } from '@/features/customers'
import { SidebarProvider } from '@/components/ui/sidebar'

function buildRouter() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  const rootRoute = createRootRoute({
    component: () => (
      <SidebarProvider>
        <Outlet />
        <Toaster />
      </SidebarProvider>
    ),
  })

  const signInRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/sign-in',
    validateSearch: (s: Record<string, unknown>) => ({
      redirect: (s.redirect as string) ?? '',
    }),
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
    beforeLoad: async ({ location }) => {
      try {
        await queryClient.ensureQueryData(meQueryOptions)
      } catch {
        throw redirect({ to: '/sign-in', search: { redirect: location.href } })
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
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
  })

  afterAll(() => {
    worker.stop()
  })

  it('guard → sign-in → customers list → create → list shows row', async () => {
    const { router, queryClient } = buildRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Initial guard redirects /customers → /sign-in
    await expect.element(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()

    await userEvent.fill(screen.getByLabelText('邮箱'), 'admin@example.com')
    await userEvent.fill(screen.getByLabelText('密码'), 'change-me-on-first-login')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    // Land on customers list, empty.
    await expect.element(screen.getByText('暂无客户')).toBeInTheDocument()

    // Open create dialog and fill.
    await userEvent.click(screen.getByRole('button', { name: '新建客户' }))
    await userEvent.fill(screen.getByLabelText('客户名称'), 'Smoke-Test-Acme')
    // Dialog has a "保存" button; form submit triggers POST via useCreateCustomer.
    await userEvent.click(screen.getByRole('button', { name: '保存' }))

    // Row appears in list.
    await expect.element(screen.getByText('Smoke-Test-Acme')).toBeInTheDocument()
  })
})
