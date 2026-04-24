import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { Country, ListEnvelope } from '../types'

export const COUNTRIES_QUERY_KEY = ['catalog', 'countries'] as const

export const countriesQueryOptions = (includeDisabled = false) =>
  queryOptions({
    queryKey: [...COUNTRIES_QUERY_KEY, { includeDisabled }] as const,
    queryFn: async (): Promise<Country[]> => {
      const res = await api.get<ListEnvelope<Country>>('/catalog/countries', {
        params: includeDisabled ? { include_disabled: true } : undefined,
      })
      return res.data.items
    },
    staleTime: 5 * 60 * 1000, // 5 min
  })

export function useCountries(includeDisabled = false) {
  return useQuery(countriesQueryOptions(includeDisabled))
}
