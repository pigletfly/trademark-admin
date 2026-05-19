import {
  describe,
  it,
  expect,
  beforeAll,
  beforeEach,
  afterAll,
  vi,
} from 'vitest'
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
  seedQuotationSubmitted,
} from '@/test-utils/msw/handlers'
import { useAuthStore } from '@/stores/auth-store'
import { __resetAuthInterceptorState } from '@/lib/api'
import { Quotations } from '@/features/quotation'
import { QuotationDetail } from '@/features/quotation/detail'
import { QuotationExportActions } from '@/features/quotation/components/quotation-export-actions'
import { NewQuotationPage } from '@/routes/_authenticated/quotations/new'
import { EditQuotationPage } from '@/routes/_authenticated/quotations/$id.edit'
import { __resetWizardStorePool } from '@/features/quotation/wizard/quotation-wizard'
import type { Quotation } from '@/features/quotation/types'

// Fixed IDs so seed + router + assertions line up.
const ADMIN_ID = '00000000-0000-0000-0000-000000000001'
const COUNTRY_CN_ID = '00000000-0000-0000-0000-000000000100'

function buildRouter(
  role: 'admin' | 'salesperson' | 'reviewer',
  initialPath: string
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
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
  const newRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/quotations/new',
    component: NewQuotationPage,
  })
  const editRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/quotations/$id/edit',
    component: EditQuotationPage,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      listRoute,
      detailRoute,
      newRoute,
      editRoute,
    ]),
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
      </QueryClientProvider>
    )
    await expect
      .element(screen.getByRole('heading', { name: '报价列表' }))
      .toBeInTheDocument()
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

    const { router, queryClient } = buildRouter(
      'admin',
      `/quotations/${quoteId}`
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
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
    await expect
      .element(screen.getByText('¥500.00').first())
      .toBeInTheDocument()

    // Approve — opens dialog, confirm without comment.
    await userEvent.click(screen.getByRole('button', { name: '通过' }))
    // Dialog's confirm button is also labelled 确认.
    await userEvent.click(screen.getByRole('button', { name: '确认' }))

    // Status badge now reads 已通过.
    await expect.element(screen.getByText('已通过').first()).toBeInTheDocument()

    // Once approved, the export actions become visible.
    await expect
      .element(screen.getByRole('button', { name: /导出 PDF/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /导出 Word/ }))
      .toBeInTheDocument()
  })
})

describe('withdraw + copy + adjust', () => {
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

  it('salesperson withdraws a submitted quotation back to draft', async () => {
    const custId = seedCustomer({ name: 'Acme 国际' })
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
    })

    const { router, queryClient } = buildRouter(
      'salesperson',
      `/quotations/${quoteId}`
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Initial submitted badge.
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()

    // Click withdraw — flips back to draft.
    await userEvent.click(screen.getByRole('button', { name: '撤回草稿' }))

    await expect.element(screen.getByText('草稿').first()).toBeInTheDocument()
    await expect
      .element(screen.getByText('报价已撤回为草稿'))
      .toBeInTheDocument()
  })

  it('reviewer adjusts a submitted snapshot and sees diff in history', async () => {
    const custId = seedCustomer({ name: 'Acme 国际' })
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      total_cny_cents: 10000,
    })

    const { router, queryClient } = buildRouter(
      'reviewer',
      `/quotations/${quoteId}`
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Make sure the submitted snapshot rendered before interacting.
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
    await expect.element(screen.getByText('application')).toBeInTheDocument()

    // Open the adjust sheet.
    await userEvent.click(screen.getByRole('button', { name: '调价' }))

    // The sheet contains one spinbutton (the amount input). Replace it.
    const amountInput = screen.getByRole('spinbutton').first()
    await userEvent.fill(amountInput, '15000')

    // Save the adjustment.
    await userEvent.click(screen.getByRole('button', { name: '保存' }))

    // Confirmation toast + new diff row in the history timeline.
    await expect.element(screen.getByText('调价已保存')).toBeInTheDocument()
    await expect
      .element(screen.getByText(/¥100\.00 → ¥150\.00/))
      .toBeInTheDocument()
  })

  it('copy lands on detail page of the new draft', async () => {
    const custId = seedCustomer({ name: 'Acme 国际' })
    const quoteId = seedQuotationSubmitted({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
    })

    const { router, queryClient } = buildRouter(
      'admin',
      `/quotations/${quoteId}`
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    // Source record shows 已提交 first.
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()

    // Click the copy button.
    await userEvent.click(screen.getByRole('button', { name: '复制报价' }))

    // Router should navigate to the new draft; detail page shows 草稿 badge.
    await expect.element(screen.getByText('草稿').first()).toBeInTheDocument()
    await expect
      .element(screen.getByText('报价已复制为新草稿'))
      .toBeInTheDocument()
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
      http.post(
        `/api/v1/quotations/${QUOTATION_ID}/export`,
        async ({ request }) => {
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
        }
      )
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

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <QuotationExportActions quotation={approvedQuotation} />
        <Toaster />
      </QueryClientProvider>
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

describe('quotation form', () => {
  beforeAll(async () => {
    await worker.start({ onUnhandledRequest: 'bypass' })
  })
  beforeEach(() => {
    resetMswState()
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
    localStorage.clear()
    __resetWizardStorePool()
  })
  afterAll(() => {
    worker.stop()
  })

  async function seedWizardPrereqs() {
    const custId = seedCustomer({ name: 'Acme 国际' })
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
      fee_item: 'application',
      amount_cny_cents: 50000,
    })
    return { custId }
  }

  async function fillRequiredQuotationForm(
    screen: Awaited<ReturnType<typeof render>>
  ) {
    await userEvent.click(screen.getByRole('combobox', { name: /客户/ }))
    await userEvent.click(screen.getByRole('option', { name: /Acme/ }))
    await userEvent.click(screen.getByRole('checkbox', { name: /第 9 类/ }))
    await userEvent.click(screen.getByRole('checkbox', { name: /中国/ }))
    await userEvent.click(screen.getByRole('checkbox', { name: /马德里/ }))
  }

  it('new form → save draft → detail keeps extended fields', async () => {
    await seedWizardPrereqs()
    const { router, queryClient } = buildRouter(
      'salesperson',
      '/quotations/new'
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await fillRequiredQuotationForm(screen)

    await expect.element(screen.getByText(/application/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '保存草稿' }))

    await expect.element(screen.getByText('草稿').first()).toBeInTheDocument()
    await expect
      .element(screen.getByText(/第 9 类 科学仪器/))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText(/马德里、单一分类/))
      .toBeInTheDocument()
    await expect.element(screen.getByText(/A 代理/)).toBeInTheDocument()
  })

  it('new form → save and submit → status becomes submitted', async () => {
    await seedWizardPrereqs()
    const { router, queryClient } = buildRouter(
      'salesperson',
      '/quotations/new'
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await fillRequiredQuotationForm(screen)

    await expect.element(screen.getByText(/application/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '保存并提交' }))
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
  })

  it('edit → change agent level → save and submit → status=submitted', async () => {
    const { custId } = await seedWizardPrereqs()
    const draftId = seedQuotationDraft({
      customer_id: custId,
      country_id: COUNTRY_CN_ID,
      service_tier: 'basic',
    })
    // Add pricing for B-agent tier so the post-edit preview finds
    // matching pricing entries and submit doesn't fail.
    seedPricingEntry({
      country_id: COUNTRY_CN_ID,
      service_tier: 'standard',
      fee_item: 'application',
      amount_cny_cents: 80000,
    })

    const { router, queryClient } = buildRouter(
      'salesperson',
      `/quotations/${draftId}/edit`
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await expect
      .element(screen.getByRole('checkbox', { name: /中国/ }))
      .toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: /第 9 类/ }))
    await userEvent.click(screen.getByRole('radio', { name: /B 代理/ }))
    await expect
      .element(screen.getByRole('radio', { name: /B 代理/ }))
      .toBeChecked()
    await expect.element(screen.getByText(/application/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: '保存并提交' }))
    await expect.element(screen.getByText('已提交').first()).toBeInTheDocument()
  })

  it('resume banner: pre-seeded localStorage → banner shows → discard clears form', async () => {
    await seedWizardPrereqs()
    localStorage.setItem(
      `quotation-wizard-draft:${ADMIN_ID}`,
      JSON.stringify({
        state: {
          currentStep: 2,
          editingId: null,
          draft: {
            customer_id: 'stale-customer',
            country_ids: [COUNTRY_CN_ID],
            nice_category_codes: [9],
            registration_methods: ['single'],
            agent_level: 'agent_b',
            info_sections: [],
            notes: 'stale notes',
          },
        },
        version: 0,
      })
    )

    const { router, queryClient } = buildRouter('admin', '/quotations/new')
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await expect.element(screen.getByText(/未完成的草稿/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /放弃/ }))
    await expect
      .element(screen.getByText(/未完成的草稿/))
      .not.toBeInTheDocument()
  })

  it('preview error: ERR_MISSING_PRICING → retry button + both saves disabled', async () => {
    seedCustomer({ name: 'Acme 国际' })

    const { router, queryClient } = buildRouter(
      'salesperson',
      '/quotations/new'
    )
    const screen = await render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )

    await fillRequiredQuotationForm(screen)

    await expect.element(screen.getByText(/暂无定价/)).toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '保存草稿' }))
      .toBeDisabled()
    await expect
      .element(screen.getByRole('button', { name: '保存并提交' }))
      .toBeDisabled()
    await expect
      .element(screen.getByRole('button', { name: /重试/ }))
      .toBeInTheDocument()
  })
})
