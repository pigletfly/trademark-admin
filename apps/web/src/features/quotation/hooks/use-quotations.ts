import { queryOptions, keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  Quotation,
  QuotationHistoryResponse,
  QuotationListQuery,
  QuotationListResponse,
} from '../types'

export const QUOTATIONS_QUERY_KEY = ['quotations'] as const

export const quotationsListQueryOptions = (query: QuotationListQuery) =>
  queryOptions({
    queryKey: [...QUOTATIONS_QUERY_KEY, 'list', query] as const,
    queryFn: async (): Promise<QuotationListResponse> => {
      const res = await api.get<QuotationListResponse>('/quotations', {
        params: {
          status: query.status || undefined,
          customer_id: query.customer_id || undefined,
          page: query.page ?? 1,
          page_size: query.page_size ?? 20,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export const quotationDetailQueryOptions = (id: string) =>
  queryOptions({
    queryKey: [...QUOTATIONS_QUERY_KEY, 'detail', id] as const,
    queryFn: async (): Promise<Quotation> => {
      const res = await api.get<Quotation>(`/quotations/${id}`)
      return res.data
    },
  })

export const quotationHistoryQueryOptions = (id: string) =>
  queryOptions({
    queryKey: [...QUOTATIONS_QUERY_KEY, 'history', id] as const,
    queryFn: async (): Promise<QuotationHistoryResponse> => {
      const res = await api.get<QuotationHistoryResponse>(`/quotations/${id}/history`)
      return res.data
    },
  })

export function useQuotationsList(query: QuotationListQuery) {
  return useQuery(quotationsListQueryOptions(query))
}

export function useQuotation(id: string) {
  return useQuery(quotationDetailQueryOptions(id))
}

export function useQuotationHistory(id: string) {
  return useQuery(quotationHistoryQueryOptions(id))
}
