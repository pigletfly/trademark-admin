import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore, type AuthUser } from './auth-store'

const sampleUser: AuthUser = {
  id: '00000000-0000-0000-0000-000000000001',
  name: 'Root Admin',
  email: 'admin@example.com',
  phone: '',
  role: 'admin',
  status: 'active',
}

describe('auth-store', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.reset()
  })

  it('starts with no user and status=unknown', () => {
    const { auth } = useAuthStore.getState()
    expect(auth.user).toBeNull()
    expect(auth.status).toBe('unknown')
  })

  it('setUser transitions to authenticated', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    const next = useAuthStore.getState().auth
    expect(next.user).toEqual(sampleUser)
    expect(next.status).toBe('authenticated')
  })

  it('markUnauthenticated clears user and flips status', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    auth.markUnauthenticated()
    const next = useAuthStore.getState().auth
    expect(next.user).toBeNull()
    expect(next.status).toBe('unauthenticated')
  })

  it('reset returns to unknown', () => {
    const { auth } = useAuthStore.getState()
    auth.setUser(sampleUser)
    auth.reset()
    const next = useAuthStore.getState().auth
    expect(next.user).toBeNull()
    expect(next.status).toBe('unknown')
  })
})
