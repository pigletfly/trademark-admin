import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  Customer,
  CreateCustomerRequest,
  UpdateCustomerRequest,
} from '../types'
import { CUSTOMERS_QUERY_KEY } from './use-customers'

function mapCustomerError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_DUPLICATE_NAME') return '已存在同名客户'
    if (err.response?.status === 403) return '没有权限操作客户'
  }
  return '请求失败，请稍后重试'
}

export function useCreateCustomer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateCustomerRequest): Promise<Customer> => {
      const res = await api.post<Customer>('/customers', body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: CUSTOMERS_QUERY_KEY })
      toast.success('客户已创建')
    },
    onError: (err) => {
      toast.error(mapCustomerError(err))
    },
  })
}

export function useUpdateCustomer() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      body: UpdateCustomerRequest
    }): Promise<Customer> => {
      const res = await api.patch<Customer>(`/customers/${args.id}`, args.body)
      return res.data
    },
    onSuccess: (data) => {
      void qc.invalidateQueries({ queryKey: CUSTOMERS_QUERY_KEY })
      qc.setQueryData(
        [...CUSTOMERS_QUERY_KEY, 'detail', data.id] as const,
        data
      )
      toast.success('客户已更新')
    },
    onError: (err) => {
      toast.error(mapCustomerError(err))
    },
  })
}
