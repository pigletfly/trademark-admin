import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  ListEnvelope,
  MadridPricingEntry,
  SingleClassPricingEntry,
} from '../types'
import { PRICING_QUERY_KEY } from './use-pricing-entries'

export const madridPricingListQueryOptions = (
  args: {
    country_id?: string
    include_base?: boolean
  } = {}
) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'madrid', args] as const,
    queryFn: async (): Promise<MadridPricingEntry[]> => {
      const res = await api.get<ListEnvelope<MadridPricingEntry>>(
        '/madrid-pricing-entries',
        {
          params: {
            country_id: args.country_id || undefined,
            include_base: args.include_base,
          },
        }
      )
      return res.data.items
    },
    staleTime: 60 * 1000,
  })

export function useMadridPricingList(
  args: {
    country_id?: string
    include_base?: boolean
  } = {}
) {
  return useQuery(madridPricingListQueryOptions(args))
}

export const singleClassPricingListQueryOptions = (
  args: {
    country_id?: string
  } = {}
) =>
  queryOptions({
    queryKey: [...PRICING_QUERY_KEY, 'single-class', args] as const,
    queryFn: async (): Promise<SingleClassPricingEntry[]> => {
      const res = await api.get<ListEnvelope<SingleClassPricingEntry>>(
        '/single-class-pricing-entries',
        {
          params: {
            country_id: args.country_id || undefined,
          },
        }
      )
      return res.data.items
    },
    staleTime: 60 * 1000,
  })

export function useSingleClassPricingList(args: { country_id?: string } = {}) {
  return useQuery(singleClassPricingListQueryOptions(args))
}
