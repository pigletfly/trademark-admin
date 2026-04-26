import { test, expect } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { readState } from '../fixtures/test-data'

test.describe.configure({ mode: 'serial' })

test('03-reviewer-journey: UI login + adjust pricing + approve', async ({ page }) => {
  const state = readState()
  expect(state.quotationId, 'quotationId missing — run 02-salesperson-journey first').toBeTruthy()

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.reviewer.email, state.reviewer.password)

  const detail = new DetailPage(page)
  await detail.goto(state.quotationId!)
  await detail.expectStatusBadge('已提交')

  // Adjust: bump first line from ¥1000 (100000 cents) to ¥1500 (150000 cents).
  await detail.adjustFirstLineAmount(150_000, `E2E adjust ${state.suffix}`)

  // After adjust the quotation is still "已提交" — adjust writes a diff
  // into history but doesn't change status. Verify by re-checking badge.
  await detail.expectStatusBadge('已提交')

  // Approve.
  await detail.approve(`E2E approve ${state.suffix}`)

  await detail.expectStatusBadge('已通过')
})
