import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { SignOutDialog } from './sign-out-dialog'

const navigate = vi.fn()
const mutate = vi.fn((_vars, opts) => opts?.onSettled?.())

const MOCK_HREF = 'https://app.test/dashboard?tab=1'

vi.mock('@/features/auth/hooks', () => ({
  useLogout: () => ({ mutate }),
}))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useNavigate: () => navigate,
    useLocation: () => ({ href: MOCK_HREF }),
  }
})

describe('SignOutDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls useLogout.mutate and navigates to sign-in with redirect', async () => {
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )
    await userEvent.click(getByRole('button', { name: /^退出登录$/ }))

    expect(mutate).toHaveBeenCalledOnce()
    expect(navigate).toHaveBeenCalledWith({
      to: '/sign-in',
      search: { redirect: MOCK_HREF },
      replace: true,
    })
  })

  it('does not call logout or navigate when Cancel is clicked', async () => {
    const { getByRole } = await render(
      <SignOutDialog open onOpenChange={vi.fn()} />
    )
    await userEvent.click(getByRole('button', { name: /^取消$/ }))

    expect(mutate).not.toHaveBeenCalled()
    expect(navigate).not.toHaveBeenCalled()
  })
})
