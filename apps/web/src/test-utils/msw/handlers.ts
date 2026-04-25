import { http, HttpResponse } from 'msw'
import type { AuthUser } from '@/stores/auth-store'

const adminUser: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Bootstrap Admin',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

let loggedIn = false

// In-memory customers store used across requests within a single test.
let customers: Array<{
  id: string
  name: string
  industry: string | null
  is_returning: boolean
  price_sensitive: boolean
  contact_name: string | null
  contact_phone: string | null
  contact_email: string | null
  notes: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []

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

// In-memory quotations store.
let quotations: Array<{
  id: string
  customer_id: string
  country_id: string
  service_tier: 'basic' | 'standard' | 'premium'
  status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'cancelled'
  snapshot: null | {
    lines: { fee_item: string; amount_cny_cents: number }[]
    total_cny_cents: number
    signature: string
  }
  total_cny_cents: number | null
  signature: string | null
  submitted_at: string | null
  reviewed_at: string | null
  reviewed_by: string | null
  review_comment: string | null
  notes: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []

// Quotation status-change history log keyed by quotation id.
let quotationHistory: Record<
  string,
  Array<{
    from_status: string
    to_status: string
    actor_id: string | null
    comment: string | null
    at: string
  }>
> = {}

function randomUUID() {
  // minimal RFC-4122 v4; browser polyfills have crypto.randomUUID too.
  const s = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
  return s
}

export const defaultHandlers = [
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = (await request.json()) as { email: string; password: string }
    if (body.email === 'admin@example.com' && body.password === 'change-me-on-first-login') {
      loggedIn = true
      return HttpResponse.json({ user: adminUser }, { status: 200 })
    }
    return HttpResponse.json(
      { code: 'ERR_INVALID_CREDENTIALS', message: 'email or password incorrect' },
      { status: 401 },
    )
  }),
  http.get('/api/v1/auth/me', () => {
    if (loggedIn) return HttpResponse.json({ user: adminUser })
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'authentication required' },
      { status: 401 },
    )
  }),
  http.post('/api/v1/auth/refresh', () => {
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'no refresh token' },
      { status: 401 },
    )
  }),
  http.post('/api/v1/auth/logout', () => {
    loggedIn = false
    return new HttpResponse(null, { status: 204 })
  }),

  // ---- customers ----
  http.get('/api/v1/customers', ({ request }) => {
    const url = new URL(request.url)
    const q = (url.searchParams.get('q') ?? '').toLowerCase()
    const page = Number(url.searchParams.get('page') ?? 1)
    const size = Number(url.searchParams.get('page_size') ?? 20)

    const filtered = q
      ? customers.filter(
          (c) =>
            c.name.toLowerCase().includes(q) ||
            (c.industry ?? '').toLowerCase().includes(q)
        )
      : customers
    const start = (page - 1) * size
    return HttpResponse.json({
      items: filtered.slice(start, start + size),
      page,
      page_size: size,
      total: filtered.length,
    })
  }),
  http.post('/api/v1/customers', async ({ request }) => {
    const body = (await request.json()) as Partial<{
      name: string
      industry: string | null
      is_returning: boolean
      price_sensitive: boolean
      contact_name: string | null
      contact_phone: string | null
      contact_email: string | null
      notes: string | null
    }>
    if (!body.name) {
      return HttpResponse.json({ code: 'ERR_INVALID_BODY', message: 'name required' }, { status: 400 })
    }
    if (customers.some((c) => c.name === body.name)) {
      return HttpResponse.json({ code: 'ERR_DUPLICATE_NAME', message: 'duplicate' }, { status: 409 })
    }
    const now = new Date().toISOString()
    const row = {
      id: randomUUID(),
      name: body.name,
      industry: body.industry ?? null,
      is_returning: body.is_returning ?? false,
      price_sensitive: body.price_sensitive ?? false,
      contact_name: body.contact_name ?? null,
      contact_phone: body.contact_phone ?? null,
      contact_email: body.contact_email ?? null,
      notes: body.notes ?? null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    customers = [row, ...customers]
    return HttpResponse.json(row, { status: 201 })
  }),
  http.get('/api/v1/customers/:id', ({ params }) => {
    const row = customers.find((c) => c.id === params.id)
    if (!row) {
      return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    }
    return HttpResponse.json(row)
  }),

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

  // ---- quotations ----
  // GET list with optional status filter.
  http.get('/api/v1/quotations', ({ request }) => {
    const url = new URL(request.url)
    const status = url.searchParams.get('status') || undefined
    let items = quotations
    if (status) items = items.filter((q) => q.status === status)
    return HttpResponse.json({
      items,
      total: items.length,
      page: 1,
      page_size: 20,
    })
  }),

  // POST create draft.
  http.post('/api/v1/quotations', async ({ request }) => {
    const body = (await request.json()) as {
      customer_id: string
      country_id: string
      service_tier: 'basic' | 'standard' | 'premium'
      notes?: string | null
    }
    const now = new Date().toISOString()
    const q = {
      id: randomUUID(),
      customer_id: body.customer_id,
      country_id: body.country_id,
      service_tier: body.service_tier,
      status: 'draft' as const,
      snapshot: null,
      total_cny_cents: null,
      signature: null,
      submitted_at: null,
      reviewed_at: null,
      reviewed_by: null,
      review_comment: null,
      notes: body.notes ?? null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    quotations.push(q)
    return HttpResponse.json(q, { status: 201 })
  }),

  http.get('/api/v1/quotations/:id', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    return HttpResponse.json(q)
  }),

  http.get('/api/v1/quotations/:id/history', ({ params }) => {
    return HttpResponse.json({ items: quotationHistory[params.id as string] ?? [] })
  }),

  http.patch('/api/v1/quotations/:id', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'draft') {
      return HttpResponse.json({ code: 'ERR_INVALID_TRANSITION' }, { status: 409 })
    }
    const body = (await request.json()) as Record<string, unknown>
    Object.assign(q, body, { updated_at: new Date().toISOString() })
    return HttpResponse.json(q)
  }),

  http.post('/api/v1/quotations/:id/submit', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'draft') {
      return HttpResponse.json({ code: 'ERR_INVALID_TRANSITION' }, { status: 409 })
    }
    // Freeze a snapshot from whatever pricing is registered for (country, tier).
    const matching = pricingEntries.filter(
      (p) => p.country_id === q.country_id && p.service_tier === q.service_tier && !p.effective_to,
    )
    if (matching.length === 0) {
      return HttpResponse.json({ code: 'ERR_MISSING_PRICING' }, { status: 422 })
    }
    const lines = matching
      .map((p) => ({ fee_item: p.fee_item, amount_cny_cents: p.amount_cny_cents }))
      .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
    const total = lines.reduce((s, l) => s + l.amount_cny_cents, 0)
    const now = new Date().toISOString()
    q.status = 'submitted'
    q.snapshot = { lines, total_cny_cents: total, signature: 'mock-sig-' + q.id.slice(0, 8) }
    q.total_cny_cents = total
    q.signature = q.snapshot.signature
    q.submitted_at = now
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'draft',
      to_status: 'submitted',
      actor_id: adminUser.id,
      comment: null,
      at: now,
    })
    return HttpResponse.json(q)
  }),

  http.post('/api/v1/quotations/:id/approve', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'submitted') {
      return HttpResponse.json({ code: 'ERR_INVALID_TRANSITION' }, { status: 409 })
    }
    const body = (await request.json().catch(() => ({}))) as { comment?: string }
    const now = new Date().toISOString()
    q.status = 'approved'
    q.reviewed_at = now
    q.reviewed_by = adminUser.id
    q.review_comment = body.comment ?? null
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'submitted',
      to_status: 'approved',
      actor_id: adminUser.id,
      comment: body.comment ?? null,
      at: now,
    })
    return HttpResponse.json(q)
  }),

  http.post('/api/v1/quotations/:id/reject', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'submitted') {
      return HttpResponse.json({ code: 'ERR_INVALID_TRANSITION' }, { status: 409 })
    }
    const body = (await request.json().catch(() => ({}))) as { comment?: string }
    const now = new Date().toISOString()
    q.status = 'rejected'
    q.reviewed_at = now
    q.reviewed_by = adminUser.id
    q.review_comment = body.comment ?? null
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'submitted',
      to_status: 'rejected',
      actor_id: adminUser.id,
      comment: body.comment ?? null,
      at: now,
    })
    return HttpResponse.json(q)
  }),

  http.post('/api/v1/quotations/:id/cancel', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'draft') {
      return HttpResponse.json({ code: 'ERR_INVALID_TRANSITION' }, { status: 409 })
    }
    const body = (await request.json().catch(() => ({}))) as { comment?: string }
    const now = new Date().toISOString()
    q.status = 'cancelled'
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'draft',
      to_status: 'cancelled',
      actor_id: adminUser.id,
      comment: body.comment ?? null,
      at: now,
    })
    return HttpResponse.json(q)
  }),

  // ---- catalog minimal handlers to satisfy sidebar / 403 cases ----
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
  http.get('/api/v1/catalog/nice-categories', () => {
    return HttpResponse.json({ items: [] })
  }),
]

export function resetMswState() {
  loggedIn = false
  customers = []
  pricingEntries = []
  quotations = []
  quotationHistory = {}
}

export function seedPricingEntry(p: {
  country_id: string
  service_tier: 'basic' | 'standard' | 'premium'
  fee_item: string
  amount_cny_cents: number
}) {
  const now = new Date().toISOString()
  pricingEntries.push({
    id: randomUUID(),
    country_id: p.country_id,
    service_tier: p.service_tier,
    fee_item: p.fee_item,
    amount_cny_cents: p.amount_cny_cents,
    notes: null,
    effective_from: now,
    effective_to: null,
    created_by: adminUser.id,
    created_at: now,
    updated_at: now,
  })
}
