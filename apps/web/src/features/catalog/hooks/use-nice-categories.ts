import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { NiceCategory, ListEnvelope } from '../types'

export const NICE_CATEGORIES_QUERY_KEY = ['catalog', 'nice-categories'] as const

export const niceCategoriesQueryOptions = queryOptions({
  queryKey: NICE_CATEGORIES_QUERY_KEY,
  queryFn: async (): Promise<NiceCategory[]> => {
    const res = await api.get<ListEnvelope<NiceCategory>>('/catalog/nice-categories')
    return res.data.items
  },
  staleTime: 60 * 60 * 1000, // 1 hour — basically static
})

export function useNiceCategories() {
  return useQuery(niceCategoriesQueryOptions)
}
