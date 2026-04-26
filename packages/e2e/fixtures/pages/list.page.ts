import { Page, expect } from '@playwright/test'

export class ListPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/quotations')
    await expect(this.page.getByRole('heading', { name: '报价列表' })).toBeVisible()
  }

  async clickNew() {
    await this.page.getByRole('link', { name: '新建报价' }).click()
    await this.page.waitForURL(/\/quotations\/new$/)
  }
}
