import { test, expect, request as pwRequest } from '@playwright/test'
import { LoginPage } from '../fixtures/pages/login.page'
import { ListPage } from '../fixtures/pages/list.page'
import { WizardPage } from '../fixtures/pages/wizard.page'
import { DetailPage } from '../fixtures/pages/detail.page'
import { patchState, readState } from '../fixtures/test-data'
import { listCountries, login } from '../fixtures/api-client'

test.describe.configure({ mode: 'serial' })

test('02-salesperson-journey: UI login + 5-step wizard + submit', async ({ page }) => {
  const state = readState()

  // We need the country code (CN/US/...) for the country-selector option
  // label. Do a quick API lookup rather than persisting it in state.
  const base = await pwRequest.newContext()
  let country: { id: string; code: string; name_zh: string } | undefined
  try {
    const session = await login(base, state.salesperson.email, state.salesperson.password)
    const countries = await listCountries(session.request)
    country = countries.find((c) => c.id === state.countryId)
    expect(country, 'seeded country not found via API').toBeTruthy()
  } finally {
    await base.dispose()
  }

  // --- UI ---
  const customerName = `客户-${state.suffix} 有限公司`

  const signIn = new LoginPage(page)
  await signIn.goto()
  await signIn.signIn(state.salesperson.email, state.salesperson.password)

  const list = new ListPage(page)
  await list.goto()
  await list.clickNew()

  const wizard = new WizardPage(page)

  // Step 1: 客户
  await wizard.expectStep(1)
  await wizard.selectCustomerByName(customerName)
  await wizard.next()

  // Step 2: 国家
  await wizard.expectStep(2)
  await wizard.selectCountryByCode(country!.code)
  await wizard.next()

  // Step 3: 级别
  await wizard.expectStep(3)
  await wizard.selectTier('basic')
  await wizard.next()

  // Step 4: 备注
  await wizard.expectStep(4)
  await wizard.fillNotes(`E2E run ${state.suffix}`)
  await wizard.next()

  // Step 5: 预览 + 提交
  await wizard.expectStep(5)
  await wizard.waitForPreview()
  const quotationId = await wizard.saveAndSubmit()

  // Verify detail page shows 已提交 badge.
  const detail = new DetailPage(page)
  await expect(page).toHaveURL(new RegExp(`/quotations/${quotationId}$`))
  await detail.expectStatusBadge('已提交')

  patchState({ quotationId })
})
