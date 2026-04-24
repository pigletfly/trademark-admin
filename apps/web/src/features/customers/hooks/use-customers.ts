import { queryOptions, keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  Customer,
  CustomerListQuery,
  CustomerListResponse,
} from '../types'

export const CUSTOMERS_QUERY_KEY = ['customers'] as const

export const customersListQueryOptions = (query: CustomerListQuery) =>
  queryOptions({
    queryKey: [...CUSTOMERS_QUERY_KEY, 'list', query] as const,
    queryFn: async (): Promise<CustomerListResponse> => {
      const res = await api.get<CustomerListResponse>('/customers', {
        params: {
          q: query.q || undefined,
          page: query.page ?? 1,
          page_size: query.page_size ?? 20,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const customerDetailQueryOptions = (id: string) =>
  queryOptions({
    queryKey: [...CUSTOMERS_QUERY_KEY, 'detail', id] as const,
    queryFn: async (): Promise<Customer> => {
      const res = await api.get<Customer>(`/customers/${id}`)
      return res.data
    },
  })

export function useCustomersList(query: CustomerListQuery) {
  return useQuery(customersListQueryOptions(query))
}

export function useCustomer(id: string) {
  return useQuery(customerDetailQueryOptions(id))
}
