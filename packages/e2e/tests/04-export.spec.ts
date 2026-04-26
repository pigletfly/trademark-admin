import { test, expect } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { readState } from '../fixtures/test-data'

test.describe.configure({ mode: 'serial' })

test('04-export: PDF bilingual export fires signed download URL', async ({ page }) => {
  const state = readState()
  expect(state.quotationId, 'quotationId missing').toBeTruthy()

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.salesperson.email, state.salesperson.password)

  const detail = new DetailPage(page)
  await detail.goto(state.quotationId!)
  // Must be approved — 03-reviewer-journey put it into 已通过.
  await detail.expectStatusBadge('已通过')

  // Set up response listener BEFORE clicking.
  const exportResponsePromise = page.waitForResponse(
    (r) =>
      r.url().includes(`/quotations/${state.quotationId}/export`) &&
      r.request().method() === 'POST',
    { timeout: 15_000 },
  )

  await detail.triggerBilingualPdfExport()

  const resp = await exportResponsePromise
  expect(resp.ok(), `export POST not ok: ${resp.status()}`).toBeTruthy()
  const body = (await resp.json()) as {
    format: string
    language: string
    download_url: string
  }
  expect(body.format).toBe('pdf')
  expect(body.language).toBe('bilingual')
  expect(body.download_url).toMatch(
    /\/api\/v1\/exports\/[0-9a-f-]{36}\/download\?token=.+/,
  )
})
