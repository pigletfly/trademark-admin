import { Page, expect } from '@playwright/test'

export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/sign-in')
    await expect(this.page.getByRole('button', { name: '登录' })).toBeVisible()
  }

  async signIn(email: string, password: string) {
    await this.page.getByLabel('邮箱').fill(email)
    // PasswordInput renders an <input type="password"> with the 密码 label.
    await this.page.getByLabel('密码').fill(password)
    await this.page.getByRole('button', { name: '登录' }).click()
    // Sign-in route redirects to "/" (safeRedirect default) on success.
    await this.page.waitForURL((url) => !url.pathname.startsWith('/sign-in'), {
      timeout: 10_000,
    })
  }
}
