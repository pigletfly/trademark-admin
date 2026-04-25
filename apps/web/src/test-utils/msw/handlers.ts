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

  // ---- catalog minimal handlers to satisfy sidebar / 403 cases ----
  http.get('/api/v1/catalog/countries', () => {
    return HttpResponse.json({ items: [] })
  }),
  http.get('/api/v1/catalog/nice-categories', () => {
    return HttpResponse.json({ items: [] })
  }),
]

export function resetMswState() {
  loggedIn = false
  customers = []
}
