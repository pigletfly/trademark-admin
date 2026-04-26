import { APIRequestContext, expect } from '@playwright/test'

const API_BASE = 'http://localhost:8080/api/v1'

// Cookie names MUST match apps/api/internal/auth/middleware.go constants.
const CSRF_COOKIE = 'tm_csrf_token'
const CSRF_HEADER = 'X-CSRF-Token'

export interface LoggedIn {
  /** APIRequestContext with cookies seeded + CSRF header baked in. */
  request: APIRequestContext
  userId: string
  role: string
}

/**
 * Log in as the given user and return a NEW APIRequestContext that carries
 * the auth + CSRF cookies, with X-CSRF-Token baked into extraHTTPHeaders.
 * Use the returned `request` for all subsequent admin-authenticated calls.
 *
 * Why a new context: Playwright's global `request` fixture shares cookies
 * across tests unless you scope it. Creating a fresh one per login keeps
 * state clean and lets us bake the CSRF header once.
 */
export async function login(
  baseRequest: APIRequestContext,
  email: string,
  password: string,
): Promise<LoggedIn> {
  // Step 1: POST /auth/login on the shared context to set cookies on its jar.
  const resp = await baseRequest.post(`${API_BASE}/auth/login`, {
    data: { email, password },
  })
  expect(
    resp.ok(),
    `login failed: ${resp.status()} ${await resp.text()}`,
  ).toBeTruthy()
  const body = (await resp.json()) as { user: { id: string; role: string } }

  // Step 2: read CSRF cookie from the jar.
  const storage = await baseRequest.storageState()
  const csrf = storage.cookies.find((c) => c.name === CSRF_COOKIE)?.value
  expect(csrf, 'CSRF cookie missing after login').toBeTruthy()

  // Step 3: return the baseRequest wrapped with the CSRF header baked into
  // every mutating call via a Proxy. The cookie jar is already seeded.
  return {
    request: createCsrfScopedRequest(baseRequest, csrf!),
    userId: body.user.id,
    role: body.user.role,
  }
}

/**
 * Wrap APIRequestContext so every non-GET request auto-includes X-CSRF-Token.
 */
function createCsrfScopedRequest(
  base: APIRequestContext,
  csrfToken: string,
): APIRequestContext {
  type Opts = Parameters<APIRequestContext['post']>[1]

  const withHeader = (opts: Opts): Opts => ({
    ...(opts ?? {}),
    headers: { ...(opts?.headers ?? {}), [CSRF_HEADER]: csrfToken },
  })

  return new Proxy(base, {
    get(target, prop, receiver) {
      if (prop === 'post' || prop === 'patch' || prop === 'delete' || prop === 'put') {
        return (url: string, opts?: Opts) =>
          (target[prop as 'post'] as APIRequestContext['post'])(url, withHeader(opts))
      }
      return Reflect.get(target, prop, receiver)
    },
  }) as APIRequestContext
}

/** POST /admin/users — requires role=admin. */
export async function createUser(
  req: APIRequestContext,
  args: { name: string; email: string; role: 'salesperson' | 'reviewer'; password: string },
): Promise<{ id: string; email: string }> {
  const r = await req.post(`${API_BASE}/admin/users`, { data: args })
  expect(r.ok(), `createUser failed: ${r.status()} ${await r.text()}`).toBeTruthy()
  const body = (await r.json()) as { user: { id: string; email: string } }
  return body.user
}

/** GET /catalog/countries — public to any authenticated user. */
export async function listCountries(
  req: APIRequestContext,
): Promise<{ id: string; code: string; name_zh: string }[]> {
  const r = await req.get(`${API_BASE}/catalog/countries`)
  expect(r.ok(), `listCountries failed: ${r.status()} ${await r.text()}`).toBeTruthy()
  const body = (await r.json()) as { items: { id: string; code: string; name_zh: string }[] }
  return body.items
}

/** POST /pricing-entries — requires role=admin. */
export async function createPricingEntry(
  req: APIRequestContext,
  args: {
    country_id: string
    service_tier: string
    fee_item: string
    amount_cny_cents: number
    effective_from: string
  },
): Promise<{ id: string }> {
  const r = await req.post(`${API_BASE}/pricing-entries`, { data: args })
  expect(
    r.ok(),
    `createPricingEntry failed: ${r.status()} ${await r.text()}`,
  ).toBeTruthy()
  return (await r.json()) as { id: string }
}

/** POST /customers — any authenticated user can write. */
export async function createCustomer(
  req: APIRequestContext,
  args: { name: string; is_returning: boolean; price_sensitive: boolean },
): Promise<{ id: string }> {
  const r = await req.post(`${API_BASE}/customers`, { data: args })
  expect(r.ok(), `createCustomer failed: ${r.status()} ${await r.text()}`).toBeTruthy()
  return (await r.json()) as { id: string }
}
