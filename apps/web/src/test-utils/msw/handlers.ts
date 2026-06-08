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

let madridPricingEntries: Array<{
  id: string
  country_id?: string | null
  sequence_no?: number | null
  country_area: string
  official_fee_chf_cents: number
  agency_fee_cny_cents: number
  is_base_fee: boolean
  notes: string | null
  effective_from: string
  effective_to: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []

let singleClassPricingEntries: Array<{
  id: string
  country_id: string
  continent: string
  country_area: string
  first_class_fee_cny_cents: number
  first_class_fee_tax6_cny_cents: number
  first_class_fee_tax1_cny_cents: number
  additional_class_fee_cny_cents: number
  additional_class_fee_tax6_cny_cents: number
  additional_class_fee_tax1_cny_cents: number
  required_documents: string
  notarization_fee: string
  acceptance_time: string
  registration_months: string
  validity_years?: number | null
  note1?: string | null
  note2?: string | null
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
  country_ids: string[]
  nice_category_codes: number[]
  registration_methods: Array<'madrid' | 'single'>
  agent_level: 'agent_a' | 'agent_b'
  service_tier: 'basic' | 'standard' | 'premium'
  status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'cancelled'
  snapshot: null | {
    lines: { fee_item: string; amount_cny_cents: number }[]
    total_cny_cents: number
    signature: string
  }
  total_cny_cents: number | null
  signature: string | null
  serial_no: string | null
  submitted_at: string | null
  reviewed_at: string | null
  reviewed_by: string | null
  review_comment: string | null
  info_sections: Array<
    | 'acceptance_time'
    | 'registration_time'
    | 'required_documents'
    | 'registration_method_intro'
    | 'real_cases'
  >
  notes: string | null
  created_by: string
  created_at: string
  updated_at: string
}> = []

type HistoryDiff = {
  lines_added?: { fee_item: string; before: number; after: number }[]
  lines_removed?: { fee_item: string; before: number; after: number }[]
  lines_updated?: { fee_item: string; before: number; after: number }[]
  total_before: number
  total_after: number
}

// Quotation status-change history log keyed by quotation id.
let quotationHistory: Record<
  string,
  Array<{
    from_status: string
    to_status: string
    actor_id: string | null
    comment: string | null
    at: string
    diff_json?: HistoryDiff | null
  }>
> = {}

function randomUUID() {
  // minimal RFC-4122 v4; browser polyfills have crypto.randomUUID too.
  const s = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
  return s
}

function calculateMethodLines(
  countryIds: string[],
  methods: Array<'madrid' | 'single'>,
  classCount: number
): null | Array<{
  fee_item: string
  amount_cny_cents: number
  source_pricing_id?: string
  source_pricing_table?: string
  registration_method?: 'madrid' | 'single'
  country_id?: string | null
  country_area?: string
  quantity?: number
  unit_amount_cny_cents?: number
  official_fee_chf_cents?: number
}> {
  const hasSingle = singleClassPricingEntries.some(
    (entry) => entry.effective_to == null
  )
  const hasMadrid = madridPricingEntries.some(
    (entry) => entry.effective_to == null
  )
  if (!hasSingle && !hasMadrid) return null

  const lines: Array<{
    fee_item: string
    amount_cny_cents: number
    source_pricing_id?: string
    source_pricing_table?: string
    registration_method?: 'madrid' | 'single'
    country_id?: string | null
    country_area?: string
    quantity?: number
    unit_amount_cny_cents?: number
    official_fee_chf_cents?: number
  }> = []
  for (const method of methods) {
    if (method === 'single') {
      for (const countryId of countryIds) {
        const row = singleClassPricingEntries.find(
          (entry) =>
            entry.effective_to == null && entry.country_id === countryId
        )
        if (!row) return []
        lines.push({
          fee_item: 'Single filing first class fee',
          amount_cny_cents: row.first_class_fee_cny_cents,
          source_pricing_id: row.id,
          source_pricing_table: 'single_class_pricing_entries',
          registration_method: 'single',
          country_id: countryId,
          country_area: row.country_area,
          quantity: 1,
          unit_amount_cny_cents: row.first_class_fee_cny_cents,
        })
        const additionalCount = Math.max(0, classCount - 1)
        if (additionalCount > 0) {
          lines.push({
            fee_item: 'Single filing additional class fee',
            amount_cny_cents:
              row.additional_class_fee_cny_cents * additionalCount,
            source_pricing_id: row.id,
            source_pricing_table: 'single_class_pricing_entries',
            registration_method: 'single',
            country_id: countryId,
            country_area: row.country_area,
            quantity: additionalCount,
            unit_amount_cny_cents: row.additional_class_fee_cny_cents,
          })
        }
      }
    }
    if (method === 'madrid') {
      const base = madridPricingEntries.find(
        (entry) => entry.effective_to == null && entry.is_base_fee
      )
      if (!base) return []
      const baseOfficial = Math.round((base.official_fee_chf_cents * 880) / 100)
      lines.push({
        fee_item: 'Madrid base official fee',
        amount_cny_cents: baseOfficial,
        source_pricing_id: base.id,
        source_pricing_table: 'madrid_pricing_entries',
        registration_method: 'madrid',
        country_area: base.country_area,
        quantity: 1,
        unit_amount_cny_cents: baseOfficial,
        official_fee_chf_cents: base.official_fee_chf_cents,
      })
      lines.push({
        fee_item: 'Madrid base agency fee',
        amount_cny_cents: base.agency_fee_cny_cents,
        source_pricing_id: base.id,
        source_pricing_table: 'madrid_pricing_entries',
        registration_method: 'madrid',
        country_area: base.country_area,
        quantity: 1,
        unit_amount_cny_cents: base.agency_fee_cny_cents,
      })
      for (const countryId of countryIds) {
        const row = madridPricingEntries.find(
          (entry) =>
            entry.effective_to == null &&
            !entry.is_base_fee &&
            entry.country_id === countryId
        )
        if (!row) return []
        const official = Math.round((row.official_fee_chf_cents * 880) / 100)
        lines.push({
          fee_item: 'Madrid designated country official fee',
          amount_cny_cents: official,
          source_pricing_id: row.id,
          source_pricing_table: 'madrid_pricing_entries',
          registration_method: 'madrid',
          country_id: countryId,
          country_area: row.country_area,
          quantity: 1,
          unit_amount_cny_cents: official,
          official_fee_chf_cents: row.official_fee_chf_cents,
        })
        lines.push({
          fee_item: 'Madrid designated country agency fee',
          amount_cny_cents: row.agency_fee_cny_cents,
          source_pricing_id: row.id,
          source_pricing_table: 'madrid_pricing_entries',
          registration_method: 'madrid',
          country_id: countryId,
          country_area: row.country_area,
          quantity: 1,
          unit_amount_cny_cents: row.agency_fee_cny_cents,
        })
      }
    }
  }
  return lines
}

export const defaultHandlers = [
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = (await request.json()) as { email: string; password: string }
    if (
      body.email === 'admin@example.com' &&
      body.password === 'change-me-on-first-login'
    ) {
      loggedIn = true
      return HttpResponse.json({ user: adminUser }, { status: 200 })
    }
    return HttpResponse.json(
      {
        code: 'ERR_INVALID_CREDENTIALS',
        message: 'email or password incorrect',
      },
      { status: 401 }
    )
  }),
  http.get('/api/v1/auth/me', () => {
    if (loggedIn) return HttpResponse.json({ user: adminUser })
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'authentication required' },
      { status: 401 }
    )
  }),
  http.post('/api/v1/auth/refresh', () => {
    return HttpResponse.json(
      { code: 'ERR_UNAUTHORIZED', message: 'no refresh token' },
      { status: 401 }
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
      return HttpResponse.json(
        { code: 'ERR_INVALID_BODY', message: 'name required' },
        { status: 400 }
      )
    }
    if (customers.some((c) => c.name === body.name)) {
      return HttpResponse.json(
        { code: 'ERR_DUPLICATE_NAME', message: 'duplicate' },
        { status: 409 }
      )
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
      .filter(
        (p) =>
          p.country_id === country &&
          p.service_tier === tier &&
          p.fee_item === fee
      )
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
  http.post(
    '/api/v1/pricing-entries/:id/deprecate',
    async ({ params, request }) => {
      const body = (await request.json().catch(() => ({}))) as {
        effective_to?: string
      }
      const row = pricingEntries.find((p) => p.id === params.id)
      if (!row)
        return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
      if (row.effective_to) {
        return HttpResponse.json(
          { code: 'ERR_ALREADY_DEPRECATED', message: 'already deprecated' },
          { status: 409 }
        )
      }
      row.effective_to =
        body.effective_to ??
        new Date(Date.now() + 86_400_000).toISOString().slice(0, 10)
      return HttpResponse.json(row)
    }
  ),

  http.get('/api/v1/madrid-pricing-entries', ({ request }) => {
    const url = new URL(request.url)
    const country = url.searchParams.get('country_id')
    const includeBase = url.searchParams.get('include_base') === 'true'
    const items = madridPricingEntries.filter((entry) => {
      if (entry.effective_to != null) return false
      if (!country) return true
      if (entry.is_base_fee) return includeBase
      return entry.country_id === country
    })
    return HttpResponse.json({ items })
  }),
  http.post('/api/v1/madrid-pricing-entries', async ({ request }) => {
    const body = (await request.json()) as {
      country_id?: string | null
      sequence_no?: number | null
      country_area: string
      official_fee_chf_cents: number
      agency_fee_cny_cents: number
      is_base_fee: boolean
      notes?: string | null
      effective_from: string
    }
    for (const entry of madridPricingEntries) {
      if (entry.effective_to != null) continue
      if (body.is_base_fee && entry.is_base_fee) {
        entry.effective_to = body.effective_from
      }
      if (
        !body.is_base_fee &&
        !entry.is_base_fee &&
        entry.country_id === body.country_id
      ) {
        entry.effective_to = body.effective_from
      }
    }
    const now = new Date().toISOString()
    const row = {
      id: 'mp_' + Math.random().toString(36).slice(2, 10),
      country_id: body.country_id ?? null,
      sequence_no: body.sequence_no ?? null,
      country_area: body.country_area,
      official_fee_chf_cents: body.official_fee_chf_cents,
      agency_fee_cny_cents: body.agency_fee_cny_cents,
      is_base_fee: body.is_base_fee,
      notes: body.notes ?? null,
      effective_from: body.effective_from,
      effective_to: null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    madridPricingEntries.push(row)
    return HttpResponse.json(row, { status: 201 })
  }),

  http.get('/api/v1/single-class-pricing-entries', ({ request }) => {
    const url = new URL(request.url)
    const country = url.searchParams.get('country_id')
    const items = singleClassPricingEntries.filter(
      (entry) =>
        entry.effective_to == null && (!country || entry.country_id === country)
    )
    return HttpResponse.json({ items })
  }),
  http.post('/api/v1/single-class-pricing-entries', async ({ request }) => {
    const body = (await request.json()) as {
      country_id: string
      continent: string
      country_area: string
      first_class_fee_cny_cents: number
      first_class_fee_tax6_cny_cents: number
      first_class_fee_tax1_cny_cents: number
      additional_class_fee_cny_cents: number
      additional_class_fee_tax6_cny_cents: number
      additional_class_fee_tax1_cny_cents: number
      required_documents?: string
      notarization_fee?: string
      acceptance_time?: string
      registration_months?: string
      validity_years?: number | null
      note1?: string | null
      note2?: string | null
      effective_from: string
    }
    for (const entry of singleClassPricingEntries) {
      if (entry.country_id === body.country_id && entry.effective_to == null) {
        entry.effective_to = body.effective_from
      }
    }
    const now = new Date().toISOString()
    const row = {
      id: 'sp_' + Math.random().toString(36).slice(2, 10),
      country_id: body.country_id,
      continent: body.continent,
      country_area: body.country_area,
      first_class_fee_cny_cents: body.first_class_fee_cny_cents,
      first_class_fee_tax6_cny_cents: body.first_class_fee_tax6_cny_cents,
      first_class_fee_tax1_cny_cents: body.first_class_fee_tax1_cny_cents,
      additional_class_fee_cny_cents: body.additional_class_fee_cny_cents,
      additional_class_fee_tax6_cny_cents:
        body.additional_class_fee_tax6_cny_cents,
      additional_class_fee_tax1_cny_cents:
        body.additional_class_fee_tax1_cny_cents,
      required_documents: body.required_documents ?? '',
      notarization_fee: body.notarization_fee ?? '',
      acceptance_time: body.acceptance_time ?? '',
      registration_months: body.registration_months ?? '',
      validity_years: body.validity_years ?? null,
      note1: body.note1 ?? null,
      note2: body.note2 ?? null,
      effective_from: body.effective_from,
      effective_to: null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    singleClassPricingEntries.push(row)
    return HttpResponse.json(row, { status: 201 })
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
      country_ids?: string[]
      nice_category_codes?: number[]
      registration_methods?: Array<'madrid' | 'single'>
      agent_level?: 'agent_a' | 'agent_b'
      service_tier: 'basic' | 'standard' | 'premium'
      info_sections?: Array<
        | 'acceptance_time'
        | 'registration_time'
        | 'required_documents'
        | 'registration_method_intro'
        | 'real_cases'
      >
      notes?: string | null
    }
    const now = new Date().toISOString()
    const countryIds = body.country_ids?.length
      ? body.country_ids
      : [body.country_id]
    const registrationMethods: Array<'madrid' | 'single'> = body
      .registration_methods?.length
      ? body.registration_methods
      : ['single']
    const q = {
      id: randomUUID(),
      customer_id: body.customer_id,
      country_id: countryIds[0],
      country_ids: countryIds,
      nice_category_codes: body.nice_category_codes ?? [],
      registration_methods: registrationMethods,
      agent_level: body.agent_level ?? 'agent_a',
      service_tier: body.service_tier,
      status: 'draft' as const,
      snapshot: null,
      total_cny_cents: null,
      signature: null,
      serial_no: null,
      submitted_at: null,
      reviewed_at: null,
      reviewed_by: null,
      review_comment: null,
      info_sections: body.info_sections ?? [],
      notes: body.notes ?? null,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    quotations.push(q)
    return HttpResponse.json(q, { status: 201 })
  }),

  // POST preview: non-persistent pricing calculation used by the
  // wizard's preview step. Mirrors the real endpoint's behaviour —
  // looks up customer, then pricing entries, returns lines + total
  // + signature. Does NOT create a quotation.
  http.post('/api/v1/quotations/preview', async ({ request }) => {
    const body = (await request.json()) as {
      customer_id: string
      country_id: string
      country_ids?: string[]
      nice_category_codes?: number[]
      registration_methods?: Array<'madrid' | 'single'>
      service_tier: 'basic' | 'standard' | 'premium'
    }
    if (!customers.find((c) => c.id === body.customer_id)) {
      return HttpResponse.json(
        { code: 'ERR_NOT_FOUND', message: 'customer not found' },
        { status: 404 }
      )
    }
    const countryIds = body.country_ids?.length
      ? body.country_ids
      : [body.country_id]
    const methodLines = calculateMethodLines(
      countryIds,
      body.registration_methods?.length
        ? body.registration_methods
        : ['single'],
      body.nice_category_codes?.length ?? 1
    )
    if (methodLines !== null) {
      if (methodLines.length === 0) {
        return HttpResponse.json(
          { code: 'ERR_MISSING_PRICING', message: 'no pricing entries' },
          { status: 422 }
        )
      }
      const total = methodLines.reduce(
        (sum, line) => sum + line.amount_cny_cents,
        0
      )
      const signature = `mock-method-${countryIds.join('-')}-${total}`
        .padEnd(64, '0')
        .slice(0, 64)
      return HttpResponse.json({
        lines: methodLines,
        total_cny_cents: total,
        signature,
      })
    }
    const matched = pricingEntries.filter(
      (e) =>
        countryIds.includes(e.country_id) &&
        e.service_tier === body.service_tier &&
        e.effective_to === null
    )
    if (matched.length === 0) {
      return HttpResponse.json(
        { code: 'ERR_MISSING_PRICING', message: 'no pricing entries' },
        { status: 422 }
      )
    }
    const lines = matched
      .map((e) => ({
        fee_item: e.fee_item,
        amount_cny_cents: e.amount_cny_cents,
        source_pricing_entry_id: e.id,
      }))
      .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
    const total = lines.reduce((s, l) => s + l.amount_cny_cents, 0)
    const signature =
      `mock-${countryIds.join('-')}-${body.service_tier}-${total}`
        .padEnd(64, '0')
        .slice(0, 64)
    return HttpResponse.json({ lines, total_cny_cents: total, signature })
  }),

  http.get('/api/v1/quotations/:id', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    return HttpResponse.json(q)
  }),

  http.get('/api/v1/quotations/:id/history', ({ params }) => {
    return HttpResponse.json({
      items: quotationHistory[params.id as string] ?? [],
    })
  }),

  http.patch('/api/v1/quotations/:id', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'draft') {
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const body = (await request.json()) as Record<string, unknown> & {
      country_ids?: string[]
    }
    const countryIds = body.country_ids?.length ? body.country_ids : undefined
    Object.assign(q, body, countryIds ? { country_id: countryIds[0] } : {}, {
      updated_at: new Date().toISOString(),
    })
    return HttpResponse.json(q)
  }),

  http.post('/api/v1/quotations/:id/submit', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'draft') {
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const methodLines = calculateMethodLines(
      q.country_ids,
      q.registration_methods,
      q.nice_category_codes.length || 1
    )
    if (methodLines !== null) {
      if (methodLines.length === 0) {
        return HttpResponse.json(
          { code: 'ERR_MISSING_PRICING' },
          { status: 422 }
        )
      }
      const total = methodLines.reduce(
        (sum, line) => sum + line.amount_cny_cents,
        0
      )
      const now = new Date().toISOString()
      q.status = 'submitted'
      q.snapshot = {
        lines: methodLines,
        total_cny_cents: total,
        signature: 'mock-method-sig-' + q.id.slice(0, 8),
      }
      q.total_cny_cents = total
      q.signature = q.snapshot.signature
      const ymd = now.slice(0, 10).replace(/-/g, '')
      q.serial_no = 'Q' + ymd + '0001'
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
    }

    // Freeze a snapshot from whatever legacy pricing is registered for (country, tier).
    const matching = pricingEntries.filter(
      (p) =>
        q.country_ids.includes(p.country_id) &&
        p.service_tier === q.service_tier &&
        !p.effective_to
    )
    if (matching.length === 0) {
      return HttpResponse.json({ code: 'ERR_MISSING_PRICING' }, { status: 422 })
    }
    const lines = matching
      .map((p) => ({
        fee_item: p.fee_item,
        amount_cny_cents: p.amount_cny_cents,
        source_pricing_entry_id: p.id,
      }))
      .sort((a, b) => a.fee_item.localeCompare(b.fee_item))
    const total = lines.reduce((s, l) => s + l.amount_cny_cents, 0)
    const now = new Date().toISOString()
    q.status = 'submitted'
    q.snapshot = {
      lines,
      total_cny_cents: total,
      signature: 'mock-sig-' + q.id.slice(0, 8),
    }
    q.total_cny_cents = total
    q.signature = q.snapshot.signature
    const ymd = now.slice(0, 10).replace(/-/g, '')
    q.serial_no = 'Q' + ymd + '0001'
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
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const body = (await request.json().catch(() => ({}))) as {
      comment?: string
    }
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
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const body = (await request.json().catch(() => ({}))) as {
      comment?: string
    }
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
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const body = (await request.json().catch(() => ({}))) as {
      comment?: string
    }
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

  // Withdraw: submitted → draft. Clears snapshot/total/signature but keeps
  // serial_no so the record retains its identity.
  http.post('/api/v1/quotations/:id/withdraw', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'submitted') {
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const now = new Date().toISOString()
    q.status = 'draft'
    q.snapshot = null
    q.total_cny_cents = null
    q.signature = null
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'submitted',
      to_status: 'draft',
      actor_id: adminUser.id,
      comment: null,
      at: now,
    })
    return HttpResponse.json(q)
  }),

  // Copy: any status → brand-new draft cloned from source. No snapshot/serial.
  http.post('/api/v1/quotations/:id/copy', ({ params }) => {
    const src = quotations.find((x) => x.id === params.id)
    if (!src)
      return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    const now = new Date().toISOString()
    const copied = {
      id: randomUUID(),
      customer_id: src.customer_id,
      country_id: src.country_id,
      country_ids: src.country_ids,
      nice_category_codes: src.nice_category_codes,
      registration_methods: src.registration_methods,
      agent_level: src.agent_level,
      service_tier: src.service_tier,
      status: 'draft' as const,
      snapshot: null,
      total_cny_cents: null,
      signature: null,
      serial_no: null,
      submitted_at: null,
      reviewed_at: null,
      reviewed_by: null,
      review_comment: null,
      info_sections: src.info_sections,
      notes: src.notes,
      created_by: adminUser.id,
      created_at: now,
      updated_at: now,
    }
    quotations.push(copied)
    return HttpResponse.json(copied, { status: 201 })
  }),

  // Adjust: rewrite a submitted quotation's snapshot + totals and append a
  // history row with a totals-only diff.
  http.post('/api/v1/quotations/:id/adjust', async ({ params, request }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'submitted') {
      return HttpResponse.json(
        { code: 'ERR_INVALID_TRANSITION' },
        { status: 409 }
      )
    }
    const body = (await request.json()) as {
      lines: { fee_item: string; amount_cny_cents: number }[]
      comment?: string
    }
    if (!Array.isArray(body.lines) || body.lines.length === 0) {
      return HttpResponse.json({ code: 'ERR_EMPTY_ADJUST' }, { status: 422 })
    }
    const totalBefore = q.total_cny_cents ?? 0
    const totalAfter = body.lines.reduce(
      (s, l) => s + (l.amount_cny_cents || 0),
      0
    )
    const now = new Date().toISOString()
    // Adjust-produced lines are "orphan" in M4 semantics — reviewer's
    // manual override has no originating pricing entry. Intentionally
    // omit source_pricing_entry_id here (field stays undefined).
    q.snapshot = {
      lines: body.lines.map((l) => ({ ...l })),
      total_cny_cents: totalAfter,
      signature: 'mock-sig-' + q.id.slice(0, 8) + '-adj',
    }
    q.total_cny_cents = totalAfter
    q.signature = q.snapshot.signature
    q.updated_at = now
    quotationHistory[q.id] = quotationHistory[q.id] ?? []
    quotationHistory[q.id].push({
      from_status: 'submitted',
      to_status: 'submitted',
      actor_id: adminUser.id,
      comment: body.comment ?? null,
      at: now,
      diff_json: { total_before: totalBefore, total_after: totalAfter },
    })
    return HttpResponse.json(q)
  }),

  http.get('/api/v1/quotations/:id/export.docx', ({ params }) => {
    const q = quotations.find((x) => x.id === params.id)
    if (!q) return HttpResponse.json({ code: 'ERR_NOT_FOUND' }, { status: 404 })
    if (q.status !== 'approved') {
      return HttpResponse.json({ code: 'ERR_NOT_APPROVED' }, { status: 422 })
    }
    // Return a tiny stub — just the magic zip bytes so a Response body
    // exists and Content-Type matches.
    const zipMagic = new Uint8Array([0x50, 0x4b, 0x03, 0x04])
    return new HttpResponse(zipMagic, {
      status: 200,
      headers: {
        'Content-Type':
          'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'Content-Disposition': `attachment; filename="quotation-${q.id.slice(0, 8)}.docx"`,
      },
    })
  }),

  http.get('/api/v1/dashboard/summary', () => {
    // Derive counts from the in-memory quotations store.
    const counts: Record<string, number> = {}
    let approvedTotal = 0
    for (const q of quotations) {
      counts[q.status] = (counts[q.status] ?? 0) + 1
      if (q.status === 'approved' && q.total_cny_cents != null) {
        approvedTotal += q.total_cny_cents
      }
    }
    const recent = [...quotations]
      .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
      .slice(0, 5)
      .map((q) => ({
        id: q.id,
        status: q.status,
        service_tier: q.service_tier,
        total_cny_cents: q.total_cny_cents,
        created_at: q.created_at,
        updated_at: q.updated_at,
      }))
    const thirtyDaysAgo = Date.now() - 30 * 24 * 3600 * 1000
    const newCusts = customers.filter(
      (c) => Date.parse(c.created_at) >= thirtyDaysAgo
    ).length

    return HttpResponse.json({
      quotations_by_status: Object.entries(counts).map(([status, count]) => ({
        status,
        count,
      })),
      approved_total_cny_cents: approvedTotal,
      new_customers_last_30_days: newCusts,
      recent_quotations: recent,
      scope: 'firm',
    })
  }),

  // ---- catalog minimal handlers to satisfy sidebar / 403 cases ----
  http.get('/api/v1/catalog/countries', () => {
    return HttpResponse.json({
      items: [
        {
          id: '00000000-0000-0000-0000-000000000100',
          code: 'CN',
          name_zh: '中国',
          name_en: 'China',
          is_madrid_member: true,
          requires_notarization: false,
          sort_order: 0,
          enabled: true,
        },
        {
          id: '00000000-0000-0000-0000-000000000101',
          code: 'US',
          name_zh: 'United States',
          name_en: 'United States',
          is_madrid_member: true,
          requires_notarization: false,
          sort_order: 1,
          enabled: true,
        },
        {
          id: '00000000-0000-0000-0000-000000000102',
          code: 'AR',
          name_zh: 'Argentina',
          name_en: 'Argentina',
          is_madrid_member: false,
          requires_notarization: false,
          sort_order: 2,
          enabled: true,
        },
      ],
    })
  }),
  http.get('/api/v1/catalog/nice-categories', () => {
    return HttpResponse.json({
      items: [
        { code: 9, name_zh: '科学仪器', name_en: 'Scientific instruments' },
        { code: 35, name_zh: '广告销售', name_en: 'Advertising and business' },
      ],
    })
  }),
]

export function resetMswState() {
  loggedIn = false
  customers = []
  pricingEntries = []
  madridPricingEntries = []
  singleClassPricingEntries = []
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

// seedCustomer pushes a customer row so components reading /customers see it.
export function seedCustomer(p: { id?: string; name?: string } = {}): string {
  const id = p.id ?? randomUUID()
  const now = new Date().toISOString()
  customers.push({
    id,
    name: p.name ?? 'Acme',
    industry: null,
    is_returning: false,
    price_sensitive: false,
    contact_name: null,
    contact_phone: null,
    contact_email: null,
    notes: null,
    created_by: adminUser.id,
    created_at: now,
    updated_at: now,
  })
  return id
}

// seedQuotationDraft pushes a draft quotation directly so tests can skip
// the form-filling UI and exercise the detail-page state-machine flow.
export function seedQuotationDraft(p: {
  id?: string
  customer_id: string
  country_id: string
  service_tier: 'basic' | 'standard' | 'premium'
}): string {
  const id = p.id ?? randomUUID()
  const now = new Date().toISOString()
  quotations.push({
    id,
    customer_id: p.customer_id,
    country_id: p.country_id,
    country_ids: [p.country_id],
    nice_category_codes: [],
    registration_methods: ['single'],
    agent_level:
      p.service_tier === 'standard' || p.service_tier === 'premium'
        ? 'agent_b'
        : 'agent_a',
    service_tier: p.service_tier,
    status: 'draft',
    snapshot: null,
    total_cny_cents: null,
    signature: null,
    serial_no: null,
    submitted_at: null,
    reviewed_at: null,
    reviewed_by: null,
    review_comment: null,
    info_sections: [],
    notes: null,
    created_by: adminUser.id,
    created_at: now,
    updated_at: now,
  })
  return id
}

// seedQuotationSubmitted creates a submitted quotation with snapshot +
// serial_no already populated, skipping the submit mutation path so tests
// can jump straight to post-submit flows (withdraw/copy/adjust).
export function seedQuotationSubmitted(p: {
  id?: string
  customer_id: string
  country_id: string
  service_tier?: 'basic' | 'standard' | 'premium'
  total_cny_cents?: number
  owner_id?: string
}): string {
  const id = p.id ?? randomUUID()
  const now = new Date().toISOString()
  const tier = p.service_tier ?? 'basic'
  const total = p.total_cny_cents ?? 10000
  // This seeded snapshot is a test-helper shortcut that doesn't look up
  // pricing_entries; treat it as a legacy-shaped snapshot and intentionally
  // omit source_pricing_entry_id (null/undefined === legacy per M4 D1).
  const lines = [{ fee_item: 'application', amount_cny_cents: total }]
  const ymd = now.slice(0, 10).replace(/-/g, '')
  quotations.push({
    id,
    customer_id: p.customer_id,
    country_id: p.country_id,
    country_ids: [p.country_id],
    nice_category_codes: [],
    registration_methods: ['single'],
    agent_level:
      tier === 'standard' || tier === 'premium' ? 'agent_b' : 'agent_a',
    service_tier: tier,
    status: 'submitted',
    snapshot: {
      lines,
      total_cny_cents: total,
      signature: 'mock-sig-' + id.slice(0, 8),
    },
    total_cny_cents: total,
    signature: 'mock-sig-' + id.slice(0, 8),
    serial_no: 'Q' + ymd + '0001',
    submitted_at: now,
    reviewed_at: null,
    reviewed_by: null,
    review_comment: null,
    info_sections: [],
    notes: null,
    created_by: p.owner_id ?? adminUser.id,
    created_at: now,
    updated_at: now,
  })
  return id
}
