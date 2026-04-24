import { describe, it, expect, beforeEach, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { useMe } from './use-me'
import { useLogin } from './use-login'
import { useLogout } from './use-logout'

const user: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Root',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

function wrap(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }
}

describe('auth hooks', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
    vi.restoreAllMocks()
  })

  it('useMe fetches and stores the user', async () => {
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { user } } as never)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

    const { result } = renderHook(() => useMe(), { wrapper: wrap(client) })
    await waitFor(() => expect(result.current.data).toEqual(user))
    expect(useAuthStore.getState().auth.status).toBe('authenticated')
  })

  it('useLogin posts credentials and updates store', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { user } } as never)
    const client = new QueryClient()

    const { result } = renderHook(() => useLogin(), { wrapper: wrap(client) })
    await result.current.mutateAsync({ email: 'admin@example.com', password: 'pw' })
    expect(useAuthStore.getState().auth.user).toEqual(user)
  })

  it('useLogout posts and resets store', async () => {
    useAuthStore.getState().auth.setUser(user)
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: {} } as never)
    const client = new QueryClient()

    const { result } = renderHook(() => useLogout(), { wrapper: wrap(client) })
    await result.current.mutateAsync()
    expect(useAuthStore.getState().auth.status).toBe('unauthenticated')
  })
})
