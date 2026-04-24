import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, type RenderResult } from 'vitest-browser-react'
import { type Locator, userEvent } from 'vitest/browser'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { UserAuthForm } from './user-auth-form'
import { useAuthStore } from '@/stores/auth-store'

const navigate = vi.fn()
const queryClient = new QueryClient()

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useNavigate: () => navigate,
  }
})

vi.mock('@/features/auth/hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/auth/hooks')>()
  return {
    ...actual,
    useLogin: () => ({
      mutate: vi.fn((data, callbacks) => {
        // Simulate successful login
        setTimeout(() => {
          const user = {
            id: '00000000-0000-0000-0000-000000000001',
            name: 'Test User',
            email: data.email,
            phone: '',
            role: 'admin' as const,
            status: 'active' as const,
          }
          callbacks.onSuccess?.(user)
        }, 50)
      }),
      isPending: false,
    }),
  }
})

describe('UserAuthForm', () => {
  describe('Rendering without redirectTo', () => {
    let screen: RenderResult
    let emailInput: Locator
    let passwordInput: Locator
    let signInButton: Locator

    beforeEach(async () => {
      vi.clearAllMocks()
      useAuthStore.getState().auth.reset()
      screen = await render(
        <QueryClientProvider client={queryClient}>
          <UserAuthForm />
        </QueryClientProvider>
      )
      emailInput = screen.getByRole('textbox', { name: /^邮箱$/i })
      passwordInput = screen.getByLabelText(/^密码$/i)
      signInButton = screen.getByRole('button', { name: /^登录$/i })
    })

    it('renders fields and submit button', async () => {
      await expect.element(emailInput).toBeInTheDocument()
      await expect.element(passwordInput).toBeInTheDocument()
      await expect.element(signInButton).toBeInTheDocument()
    })

    it('shows validation messages when submitting empty form', async () => {
      await userEvent.click(signInButton)

      await expect
        .element(screen.getByText('请输入邮箱'))
        .toBeInTheDocument()
      await expect
        .element(screen.getByText('请输入密码'))
        .toBeInTheDocument()
    })

    it('navigates to default route on successful login', async () => {
      await userEvent.fill(emailInput, 'admin@example.com')
      await userEvent.fill(passwordInput, 'password123')

      await userEvent.click(signInButton)

      await vi.waitFor(() =>
        expect(navigate).toHaveBeenCalledWith({ to: '/', replace: true })
      )
    })
  })

  it('navigates to redirectTo when provided', async () => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.reset()

    const { getByRole, getByLabelText } = await render(
      <QueryClientProvider client={queryClient}>
        <UserAuthForm redirectTo='/settings' />
      </QueryClientProvider>
    )

    await userEvent.fill(getByRole('textbox', { name: /邮箱/i }), 'admin@example.com')
    await userEvent.fill(getByLabelText('密码'), 'password123')

    await userEvent.click(getByRole('button', { name: /登录/i }))

    await vi.waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({
        to: '/settings',
        replace: true,
      })
    )
  })
})
