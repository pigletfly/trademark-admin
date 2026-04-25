import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { ListEnvelope, PricingEntry, ServiceTier } from '../types'

export const PRICING_QUERY_KEY = ['pricing'] as const

interface ListArgs {
  country_id?: string
  service_tier?: ServiceTier
}

export const pricingListQueryOptions = (args: ListArgs = {}) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'list', args] as const,
    queryFn: async (): Promise<PricingEntry[]> => {
      const res = await api.get<ListEnvelope<PricingEntry>>('/pricing-entries', {
        params: {
          country_id: args.country_id || undefined,
          service_tier: args.service_tier || undefined,
        },
      })
      return res.data.items
    },
    staleTime: 60 * 1000,
  })

export function usePricingList(args: ListArgs = {}) {
  return useQuery(pricingListQueryOptions(args))
}
