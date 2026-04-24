import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { ME_QUERY_KEY } from './me-query'

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.post('/auth/logout')
    },
    onSuccess: () => {
      useAuthStore.getState().auth.markUnauthenticated()
      qc.removeQueries({ queryKey: ME_QUERY_KEY })
    },
  })
}
