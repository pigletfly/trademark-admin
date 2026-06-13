import type { AxiosError } from 'axios'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { RegistrationMethod, ServiceTier } from '../../types'

export interface PreviewRequest {
  customer_id: string
  country_id: string
  country_ids?: string[]
  madrid_country_ids?: string[]
  single_country_ids?: string[]
  nice_category_codes?: number[]
  registration_methods?: RegistrationMethod[]
  service_tier: ServiceTier
}

export interface PreviewLine {
  fee_item: string
  amount_cny_cents: number
  registration_method?: RegistrationMethod
  country_area?: string
}

export interface PreviewResponse {
  lines: PreviewLine[]
  total_cny_cents: number
  signature: string
}

export const PREVIEW_QUERY_KEY = ['quotations', 'preview'] as const

// usePreview fetches pricing for the wizard's current triple. Returns
// an idle/empty state when any required field is missing. Cached 5
// minutes so a round-trip to an earlier step + back doesn't hammer
// the API.
export function usePreview(req: PreviewRequest) {
  const enabled = Boolean(req.customer_id && req.country_id && req.service_tier)
  return useQuery<PreviewResponse, AxiosError>({
    queryKey: [...PREVIEW_QUERY_KEY, req],
    queryFn: async () => {
      const { data } = await api.post<PreviewResponse>(
        '/quotations/preview',
        req
      )
      return data
    },
    enabled,
    staleTime: 5 * 60_000,
    retry: false,
  })
}
