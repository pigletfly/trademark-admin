import { test, expect, request as pwRequest } from '@playwright/test'
import {
  bootstrapAdminCreds,
  randomHex,
  reviewerEmail,
  salespersonEmail,
  testCustomerPayload,
  testPricingPayload,
  TEST_PASSWORD,
  writeState,
} from '../fixtures/test-data'
import {
  createCustomer,
  createPricingEntry,
  createUser,
  listCountries,
  login,
} from '../fixtures/api-client'

test.describe.configure({ mode: 'serial' })

test('01-admin-setup: bootstrap admin seeds 2 users + 2 pricing + 1 customer', async () => {
  const suffix = randomHex(4)

  // Dedicated request context for this spec — independent cookie jar.
  const base = await pwRequest.newContext()
  const admin = bootstrapAdminCreds()
  const adminSession = await login(base, admin.email, admin.password)

  // 1. Create salesperson + reviewer.
  const salesperson = await createUser(adminSession.request, {
    name: `E2E Salesperson ${suffix}`,
    email: salespersonEmail(suffix),
    role: 'salesperson',
    password: TEST_PASSWORD,
  })
  const reviewer = await createUser(adminSession.request, {
    name: `E2E Reviewer ${suffix}`,
    email: reviewerEmail(suffix),
    role: 'reviewer',
    password: TEST_PASSWORD,
  })

  // 2. Pick any seeded country (seeder inserts ~60).
  const countries = await listCountries(adminSession.request)
  expect(countries.length, 'expected >=1 country from seeder').toBeGreaterThan(0)
  const country = countries[0]

  // 3. Two pricing entries (basic tier, application + agent fees).
  const applicationEntry = await createPricingEntry(
    adminSession.request,
    testPricingPayload(country.id, suffix, 'application', 100_000), // ¥1000
  )
  const agentEntry = await createPricingEntry(
    adminSession.request,
    testPricingPayload(country.id, suffix, 'agent', 50_000), // ¥500
  )

  // 4. Customer.
  const customer = await createCustomer(
    adminSession.request,
    testCustomerPayload(suffix),
  )

  // 5. Write state for downstream specs.
  writeState({
    suffix,
    adminEmail: admin.email,
    adminPassword: admin.password,
    salesperson: {
      id: salesperson.id,
      email: salesperson.email,
      password: TEST_PASSWORD,
    },
    reviewer: {
      id: reviewer.id,
      email: reviewer.email,
      password: TEST_PASSWORD,
    },
    customerId: customer.id,
    countryId: country.id,
    pricingEntryIds: {
      application: applicationEntry.id,
      agent: agentEntry.id,
    },
    quotationId: null,
  })

  await base.dispose()
})
