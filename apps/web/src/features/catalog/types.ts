export interface Country {
  id: string
  code: string
  name_zh: string
  name_en: string
  is_madrid_member: boolean
  default_acceptance_days?: number | null
  default_registration_months?: number | null
  requires_notarization: boolean
  notes_zh?: string | null
  notes_en?: string | null
  sort_order: number
  enabled: boolean
}

export interface NiceCategory {
  code: number
  name_zh: string
  name_en: string
  description_zh?: string | null
  description_en?: string | null
}

export interface UpdateCountryRequest {
  name_zh?: string
  name_en?: string
  is_madrid_member?: boolean
  default_acceptance_days?: number | null
  default_registration_months?: number | null
  requires_notarization?: boolean
  notes_zh?: string | null
  notes_en?: string | null
  sort_order?: number
  enabled?: boolean
}

export interface ListEnvelope<T> {
  items: T[]
}
