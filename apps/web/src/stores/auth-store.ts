import { create } from 'zustand'

export type AuthStatus = 'unknown' | 'authenticated' | 'unauthenticated'

export interface AuthUser {
  id: string
  name: string
  email: string
  phone: string
  role: 'salesperson' | 'reviewer' | 'admin'
  status: 'active' | 'disabled'
}

interface AuthState {
  auth: {
    user: AuthUser | null
    status: AuthStatus
    setUser: (user: AuthUser) => void
    markUnauthenticated: () => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    status: 'unknown',
    setUser: (user) =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user, status: 'authenticated' },
      })),
    markUnauthenticated: () =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user: null, status: 'unauthenticated' },
      })),
    reset: () =>
      set((state) => ({
        ...state,
        auth: { ...state.auth, user: null, status: 'unknown' },
      })),
  },
}))
