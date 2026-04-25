export type ServiceTier = 'basic' | 'standard' | 'premium'

export const SERVICE_TIERS: ServiceTier[] = ['basic', 'standard', 'premium']

// Friendly Chinese labels for tiers.
export const SERVICE_TIER_LABEL_ZH: Record<ServiceTier, string> = {
  basic: '基础',
  standard: '标准',
  premium: '高级',
}

export interface PricingEntry {
  id: string
  country_id: string
  service_tier: ServiceTier
  fee_item: string
  amount_cny_cents: number
  notes?: string | null
  effective_from: string
  effective_to?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateOrReplacePricingRequest {
  country_id: string
  service_tier: ServiceTier
  fee_item: string
  amount_cny_cents: number
  notes?: string | null
  effective_from: string
}

export interface ListEnvelope<T> {
  items: T[]
}
