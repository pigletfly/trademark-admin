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
import { SignIn } from '.'
import { meQueryOptions } from '@/features/auth/hooks'
import { __resetAuthInterceptorState } from '@/lib/api'

function buildRouter() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  const rootRoute = createRootRoute({
    component: () => (
      <>
        <Outlet />
        <Toaster />
      </>
    ),
  })

  const signInRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/sign-in',
    validateSearch: (search: Record<string, unknown>) => ({
      redirect: (search.redirect as string) ?? '',
    }),
    component: SignIn,
  })

function HomeComponent() {
    const user = useAuthStore((s) => s.auth.user)
    return <div>Welcome {user?.name}</div>
  }

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
    component: HomeComponent,
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

  it('unauthenticated root → sign-in → successful login → welcome', async () => {
    const { router, queryClient } = buildRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Guard should redirect to /sign-in
    await expect.element(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()

    await userEvent.fill(screen.getByLabelText('邮箱'), 'admin@example.com')
    await userEvent.fill(screen.getByLabelText('密码'), 'change-me-on-first-login')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    await expect.element(screen.getByText('Welcome Bootstrap Admin')).toBeInTheDocument()
  })

  it('wrong credentials show error toast and stay on sign-in', async () => {
    const { router, queryClient } = buildRouter()
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    await expect.element(screen.getByRole('button', { name: '登录' })).toBeInTheDocument()
    await userEvent.fill(screen.getByLabelText('邮箱'), 'admin@example.com')
    await userEvent.fill(screen.getByLabelText('密码'), 'wrong-password')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    await expect.element(screen.getByText('邮箱或密码错误')).toBeInTheDocument()
    await expect.element(screen.getByText(/Welcome/)).not.toBeInTheDocument()
  })
})
