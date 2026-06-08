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

export interface MadridPricingEntry {
  id: string
  country_id?: string | null
  sequence_no?: number | null
  country_area: string
  official_fee_chf_cents: number
  agency_fee_cny_cents: number
  is_base_fee: boolean
  notes?: string | null
  effective_from: string
  effective_to?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateOrReplaceMadridPricingRequest {
  country_id?: string | null
  sequence_no?: number | null
  country_area: string
  official_fee_chf_cents: number
  agency_fee_cny_cents: number
  is_base_fee: boolean
  notes?: string | null
  effective_from: string
}

export interface SingleClassPricingEntry {
  id: string
  country_id: string
  continent: string
  country_area: string
  first_class_fee_cny_cents: number
  first_class_fee_tax6_cny_cents: number
  first_class_fee_tax1_cny_cents: number
  additional_class_fee_cny_cents: number
  additional_class_fee_tax6_cny_cents: number
  additional_class_fee_tax1_cny_cents: number
  required_documents: string
  notarization_fee: string
  acceptance_time: string
  registration_months: string
  validity_years?: number | null
  note1?: string | null
  note2?: string | null
  effective_from: string
  effective_to?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateOrReplaceSingleClassPricingRequest {
  country_id: string
  continent: string
  country_area: string
  first_class_fee_cny_cents: number
  first_class_fee_tax6_cny_cents: number
  first_class_fee_tax1_cny_cents: number
  additional_class_fee_cny_cents: number
  additional_class_fee_tax6_cny_cents: number
  additional_class_fee_tax1_cny_cents: number
  required_documents: string
  notarization_fee: string
  acceptance_time: string
  registration_months: string
  validity_years?: number | null
  note1?: string | null
  note2?: string | null
  effective_from: string
}
