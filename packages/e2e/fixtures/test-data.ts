import { randomBytes } from 'node:crypto'
import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const STATE_PATH = path.resolve(__dirname, '..', 'state', 'test-run.json')

// 8-hex suffix (~4 billion values). Good enough to avoid collisions even if
// you rerun the E2E 100+ times without `docker compose down -v`.
export function randomHex(bytes = 4): string {
  return randomBytes(bytes).toString('hex')
}

export interface TestState {
  suffix: string
  adminEmail: string
  adminPassword: string
  salesperson: { id: string; email: string; password: string }
  reviewer: { id: string; email: string; password: string }
  customerId: string
  countryId: string
  // Recorded for human inspection only — the wizard resolves active pricing
  // by (country_id, service_tier) server-side, so downstream specs don't
  // read these ids.
  pricingEntryIds: { application: string; agent: string }
  quotationId?: string | null
}

export function bootstrapAdminCreds() {
  return {
    email: 'admin@example.com',
    password: 'change-me-on-first-login',
  }
}

export function salespersonEmail(suffix: string) {
  return `salesperson-${suffix}@example.com`
}
export function reviewerEmail(suffix: string) {
  return `reviewer-${suffix}@example.com`
}
export const TEST_PASSWORD = 'e2e-pass-word!!' // >= 8 chars — admin create validator requires it

export function testCustomerPayload(suffix: string) {
  return {
    name: `客户-${suffix} 有限公司`,
    is_returning: false,
    price_sensitive: false,
  }
}

export function testPricingPayload(
  countryId: string,
  suffix: string,
  feeItem: 'application' | 'agent',
  amountCents: number,
) {
  return {
    country_id: countryId,
    service_tier: 'basic',
    fee_item: `e2e-${feeItem}-${suffix}`,
    amount_cny_cents: amountCents,
    effective_from: '2020-01-01', // far enough in the past to always be active
  }
}

export function writeState(s: TestState): void {
  writeFileSync(STATE_PATH, JSON.stringify(s, null, 2), 'utf8')
}

export function readState(): TestState {
  if (!existsSync(STATE_PATH)) {
    throw new Error(
      `state/test-run.json not found at ${STATE_PATH}. Run 01-admin-setup spec first.`,
    )
  }
  return JSON.parse(readFileSync(STATE_PATH, 'utf8')) as TestState
}

export function patchState(patch: Partial<TestState>): TestState {
  const cur = readState()
  const next = { ...cur, ...patch }
  writeState(next)
  return next
}
