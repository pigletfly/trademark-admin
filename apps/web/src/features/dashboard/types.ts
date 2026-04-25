// Mirrors apps/api/internal/dashboard/dto.go.
import type { QuotationStatus } from '@/features/quotation/types'

export interface QuotationStatusCount {
  status: QuotationStatus
  count: number
}

export interface RecentQuotation {
  id: string
  status: QuotationStatus
  service_tier: string
  total_cny_cents?: number | null
  created_at: string
  updated_at: string
}

export interface DashboardSummary {
  quotations_by_status: QuotationStatusCount[]
  approved_total_cny_cents: number
  new_customers_last_30_days: number
  recent_quotations: RecentQuotation[]
  scope: 'self' | 'firm'
}
