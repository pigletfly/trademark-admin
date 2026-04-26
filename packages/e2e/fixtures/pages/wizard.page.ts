import { Page, expect } from '@playwright/test'

export class WizardPage {
  constructor(private page: Page) {}

  async expectStep(n: 1 | 2 | 3 | 4 | 5) {
    await expect(
      this.page.getByText(new RegExp(`第\\s*${n}\\s*步`)),
    ).toBeVisible()
  }

  /**
   * Shadcn Select is a Radix Popover, not a native <select>. We click the
   * trigger to open, then click the option by its visible text.
   */
  async selectCustomerByName(name: string) {
    await this.page.locator('#wizard-customer').click()
    await this.page.getByRole('option', { name }).click()
  }

  async selectCountryByCode(code: string) {
    await this.page.locator('#wizard-country').click()
    // Options render as "中国（CN）" — we match by the code in parens.
    await this.page.getByRole('option', { name: new RegExp(`（${code}）`) }).click()
  }

  async selectTier(tier: 'basic' | 'standard' | 'premium') {
    // RadioGroupItem has id `tier-${value}`. Label wraps it so clicking the
    // option text also works; we target the radio directly for reliability.
    await this.page.locator(`#tier-${tier}`).click()
  }

  async fillNotes(notes: string) {
    await this.page.locator('#wizard-notes').fill(notes)
  }

  async next() {
    await this.page.getByRole('button', { name: '下一步' }).click()
  }

  async back() {
    await this.page.getByRole('button', { name: '上一步' }).click()
  }

  /** On preview step: click 保存并提交 and wait for redirect to /quotations/<id>. */
  async saveAndSubmit(): Promise<string> {
    await this.page.getByRole('button', { name: '保存并提交' }).click()
    await this.page.waitForURL(/\/quotations\/[0-9a-f-]{36}$/, { timeout: 15_000 })
    const m = this.page.url().match(/\/quotations\/([0-9a-f-]{36})$/)
    expect(m, 'could not extract quotation id from URL').toBeTruthy()
    return m![1]
  }

  /**
   * Preview step renders total + signature. Wait for its appearance to be
   * sure the preview mutation finished.
   */
  async waitForPreview() {
    await expect(this.page.getByText('明细 / Line items')).toBeVisible({
      timeout: 10_000,
    })
    // Button becomes enabled only after preview succeeds.
    await expect(this.page.getByRole('button', { name: '保存并提交' })).toBeEnabled()
  }
}
