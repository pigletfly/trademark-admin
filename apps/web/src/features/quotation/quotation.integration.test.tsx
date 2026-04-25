import { describe, it, expect, beforeAll, beforeEach, afterAll, vi } from 'vitest'
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
import { http, HttpResponse } from 'msw'
import { SidebarProvider } from '@/components/ui/sidebar'
import { worker } from '@/test-utils/msw/server'
import {
  resetMswState,
  seedCustomer,
  seedPricingEntry,
  seedQuotationDraft,
} from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { Quotations } from '@/features/quotation'
import { QuotationDetail } from '@/features/quotation/detail'
import { QuotationExportActions } from '@/features/quotation/components/quotation-export-actions'
import type { Quotation } from '@/features/quotation/types'

// Fixed IDs so seed + router + assertions line up.
const ADMIN_ID = '00000000-0000-0000-0000-000000000001'
const COUNTRY_CN_ID = '00000000-0000-0000-0000-000000000100'

function buildRouter(role: 'admin' | 'salesperson' | 'reviewer', initialPath: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const user = {
    id: ADMIN_ID,
    name: 'Bootstrap Admin',
    email: 'admin@example.com',
    phone: '',
    role,
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
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/quotations',
    validateSearch: (s: Record<string, unknown>) => ({
      status: s.status as string | undefined,
    }),
    component: Quotations,
  })
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/quotations/$id',
    component: QuotationDetail,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([listRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: { queryClient },
  })
  return { router, queryClient }
}

describe('quotation integration', () => {
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

  it('renders empty list state when no quotations exist', async () => {
    const { router, queryClient } = buildRouter('admin', '/quotations')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )
    await expect.element(screen.getByRole('heading', { name: '报价列表' })).toBeInTheDocument()
    await expect.element(screen.getByText(/暂无报价记录/)).toBeInTheDocument()
  })

  it('admin submits a draft → sees frozen snapshot → approves', async () => {
    // Seed backing data: a pricing entry (¥500.00 application fee),
    // a customer, and a draft quotation owned by admin.
    const custId = seedCustomer({ name: 'Acme 国际' })
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
      fee_item: 'application',
      amount_cny_cents: 50000,
    })
    const quoteId = seedQuotationDraft({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
    })

    const { router, queryClient } = buildRouter('admin', `/quotations/${quoteId}`)
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    )

    // Draft view: status badge is "草稿", snapshot hint is shown.
    await expect.element(screen.getByText('草稿').first()).toBeInTheDocument()
    await expect.element(screen.getByText(/草稿尚未提交/)).toBeInTheDocument()

    // Submit triggers MSW /submit which freezes the ¥500.00 line.
    await userEvent.click(screen.getByRole('button', { name: '提交审核' }))

    // After submit: status badge flips to 已提交 and the snapshot
    // table shows the seeded ¥500.00 as a total cell.
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
    await expect.element(screen.getByText('application')).toBeInTheDocument()
    await expect.element(screen.getByText('¥500.00').first()).toBeInTheDocument()

    // Approve — opens dialog, confirm without comment.
    await userEvent.click(screen.getByRole('button', { name: '通过' }))
    // Dialog's confirm button is also labelled 确认.
    await userEvent.click(screen.getByRole('button', { name: '确认' }))

    // Status badge now reads 已通过.
    await expect.element(screen.getByText('已通过').first()).toBeInTheDocument()

    // Once approved, the export actions become visible.
    await expect.element(screen.getByRole('button', { name: /导出 PDF/ })).toBeInTheDocument()
    await expect.element(screen.getByRole('button', { name: /导出 Word/ })).toBeInTheDocument()
  })
})

describe('QuotationExportActions', () => {
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

  it('calls window.open with signed download_url after clicking PDF bilingual', async () => {
    const QUOTATION_ID = 'test-quotation-export-1'
    const DOWNLOAD_URL = '/api/v1/exports/exp-1/download?token=abc'

    // Capture the request body sent to the export endpoint.
    let capturedBody: unknown = null

    worker.use(
      http.post(`/api/v1/quotations/${QUOTATION_ID}/export`, async ({ request }) => {
        capturedBody = await request.json()
        const now = new Date().toISOString()
        const dto = {
          id: 'exp-1',
          quotation_id: QUOTATION_ID,
          format: 'pdf',
          language: 'bilingual',
          sha256: 'abc123',
          file_size: 1024,
          expires_at: now,
          created_at: now,
          download_url: DOWNLOAD_URL,
        }
        return HttpResponse.json(dto, { status: 201 })
      }),
    )

    // Spy on window.open before rendering.
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

    const approvedQuotation: Quotation = {
      id: QUOTATION_ID,
      customer_id: 'cust-1',
      country_id: 'country-1',
      service_tier: 'basic',
      status: 'approved',
      created_by: 'user-1',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <QuotationExportActions quotation={approvedQuotation} />
        <Toaster />
      </QueryClientProvider>,
    )

    // Click the PDF dropdown trigger button.
    await userEvent.click(screen.getByRole('button', { name: /导出 PDF/ }))

    // Click the bilingual menu item.
    await userEvent.click(screen.getByText('中英双语 / Bilingual'))

    // Wait for mutation to settle by checking window.open was called.
    await expect.poll(() => openSpy.mock.calls.length).toBeGreaterThan(0)

    expect(openSpy).toHaveBeenCalledWith(DOWNLOAD_URL, '_blank', 'noopener')
    expect(capturedBody).toEqual({ format: 'pdf', language: 'bilingual' })

    openSpy.mockRestore()
  })
})
