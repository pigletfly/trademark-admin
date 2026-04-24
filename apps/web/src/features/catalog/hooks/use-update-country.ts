import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type { Country, UpdateCountryRequest } from '../types'
import { COUNTRIES_QUERY_KEY } from './use-countries'

export function useUpdateCountry() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; body: UpdateCountryRequest }): Promise<Country> => {
      const res = await api.patch<Country>(`/catalog/countries/${args.id}`, args.body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: COUNTRIES_QUERY_KEY })
      toast.success('国家信息已更新')
    },
    onError: (err) => {
      if (err instanceof AxiosError && err.response?.status === 403) {
        toast.error('没有权限修改字典')
        return
      }
      toast.error('更新失败，请稍后重试')
    },
  })
}
