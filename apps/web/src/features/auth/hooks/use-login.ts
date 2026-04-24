import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { ME_QUERY_KEY } from './me-query'

export interface LoginInput {
  email: string
  password: string
}

interface LoginResponse {
  user: AuthUser
}

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (input: LoginInput) => {
      const { data } = await api.post<LoginResponse>('/auth/login', input)
      return data.user
    },
    onSuccess: (user) => {
      useAuthStore.getState().auth.setUser(user)
      qc.setQueryData(ME_QUERY_KEY, user)
    },
  })
}
