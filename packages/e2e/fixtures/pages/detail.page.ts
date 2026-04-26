import { Page, Request, expect } from '@playwright/test'

export class DetailPage {
  constructor(private page: Page) {}

  async goto(id: string) {
    await this.page.goto(`/quotations/${id}`)
    await expect(this.page.getByText('状态变更')).toBeVisible()
  }

  async expectStatusBadge(label: '草稿' | '已提交' | '已通过' | '已驳回' | '已取消') {
    // QuotationStatusBadge renders the Chinese label as its text.
    await expect(this.page.getByText(label, { exact: true }).first()).toBeVisible()
  }

  /** Open 调价 sheet, replace first row's amount, save. */
  async adjustFirstLineAmount(newAmountCents: number, comment?: string) {
    await this.page.getByRole('button', { name: '调价' }).click()
    // Sheet title acts as open confirmation.
    await expect(
      this.page.getByRole('heading', { name: '调价' }),
    ).toBeVisible()
    // First numeric input in the sheet is the first line's amount.
    const amountInputs = this.page.locator('input[type="number"]')
    await amountInputs.first().fill(String(newAmountCents))
    if (comment) {
      await this.page.locator('#adjust-comment').fill(comment)
    }
    // Sheet 的确认按钮文案是 "保存"。
    await this.page.getByRole('button', { name: '保存', exact: true }).click()
    // Wait for sheet to close: title gone.
    await expect(
      this.page.getByRole('heading', { name: '调价' }),
    ).toBeHidden({ timeout: 10_000 })
  }

  /** Click 通过 → confirm dialog → 确认. */
  async approve(comment?: string) {
    await this.page.getByRole('button', { name: '通过', exact: true }).click()
    await expect(
      this.page.getByRole('heading', { name: '确认通过' }),
    ).toBeVisible()
    if (comment) {
      await this.page.locator('#comment').fill(comment)
    }
    await this.page.getByRole('button', { name: '确认', exact: true }).click()
    await expect(
      this.page.getByRole('heading', { name: '确认通过' }),
    ).toBeHidden({ timeout: 10_000 })
  }

  /**
   * Click Export PDF dropdown → 中英双语 item, and return the Promise that
   * resolves when the POST /quotations/:id/export request fires.
   *
   * We don't wait for the download itself — `window.open()` in a new tab
   * doesn't necessarily trigger a `download` event, and the spec's intent
   * is to verify the signed URL is generated server-side.
   */
  async triggerBilingualPdfExport(): Promise<Request> {
    // Set up request listener BEFORE clicking.
    const reqPromise = this.page.waitForRequest(
      (req) =>
        req.url().includes('/quotations/') &&
        req.url().endsWith('/export') &&
        req.method() === 'POST',
      { timeout: 15_000 },
    )
    await this.page
      .getByRole('button', { name: /导出 PDF/ })
      .click()
    await this.page.getByRole('menuitem', { name: /中英双语/ }).click()
    return await reqPromise
  }

}
