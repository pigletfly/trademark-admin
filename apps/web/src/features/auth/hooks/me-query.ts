import { queryOptions } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

interface MeResponse {
  user: AuthUser
}

export const ME_QUERY_KEY = ['auth', 'me'] as const

export const meQueryOptions = queryOptions({
  queryKey: ME_QUERY_KEY,
  queryFn: async () => {
    const { data } = await api.get<MeResponse>('/auth/me')
    useAuthStore.getState().auth.setUser(data.user)
    return data.user
  },
  staleTime: 60 * 1000,
  retry: false,
})
