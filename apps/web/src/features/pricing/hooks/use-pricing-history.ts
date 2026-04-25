import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { ListEnvelope, PricingEntry, ServiceTier } from '../types'
import { PRICING_QUERY_KEY } from './use-pricing-entries'

interface HistoryArgs {
  country_id: string
  service_tier: ServiceTier
  fee_item: string
}

export const pricingHistoryQueryOptions = (args: HistoryArgs) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'history', args] as const,
    queryFn: async (): Promise<PricingEntry[]> => {
      const res = await api.get<ListEnvelope<PricingEntry>>('/pricing-entries/history', {
        params: args,
      })
      return res.data.items
    },
    enabled: !!args.country_id && !!args.service_tier && !!args.fee_item,
  })

export function usePricingHistory(args: HistoryArgs) {
  return useQuery(pricingHistoryQueryOptions(args))
}
