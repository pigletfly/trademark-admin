export interface Customer {
  id: string
  name: string
  industry?: string | null
  is_returning: boolean
  price_sensitive: boolean
  contact_name?: string | null
  contact_phone?: string | null
  contact_email?: string | null
  notes?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateCustomerRequest {
  name: string
  industry?: string | null
  is_returning?: boolean
  price_sensitive?: boolean
  contact_name?: string | null
  contact_phone?: string | null
  contact_email?: string | null
  notes?: string | null
}

export type UpdateCustomerRequest = Partial<CreateCustomerRequest>

export interface CustomerListResponse {
  items: Customer[]
  page: number
  page_size: number
  total: number
}

export interface CustomerListQuery {
  q?: string
  page?: number
  page_size?: number
}
