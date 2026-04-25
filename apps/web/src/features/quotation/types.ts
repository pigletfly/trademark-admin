// Mirrors the Go DTOs in apps/api/internal/quotation/dto.go.
export type QuotationStatus = 'draft' | 'submitted' | 'approved' | 'rejected' | 'cancelled'

export const QUOTATION_STATUS_LABEL_ZH: Record<QuotationStatus, string> = {
  draft: '草稿',
  submitted: '已提交',
  approved: '已通过',
  rejected: '已驳回',
  cancelled: '已取消',
}

export type ServiceTier = 'basic' | 'standard' | 'premium'

export interface SnapshotLine {
  fee_item: string
  amount_cny_cents: number
}

export interface QuotationSnapshot {
  lines: SnapshotLine[]
  total_cny_cents: number
  signature: string
}

export interface Quotation {
  id: string
  customer_id: string
  country_id: string
  service_tier: ServiceTier
  status: QuotationStatus
  snapshot?: QuotationSnapshot
  total_cny_cents?: number | null
  signature?: string | null
  submitted_at?: string | null
  reviewed_at?: string | null
  reviewed_by?: string | null
  review_comment?: string | null
  notes?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateQuotationRequest {
  customer_id: string
  country_id: string
  service_tier: ServiceTier
  notes?: string | null
}

export interface UpdateDraftRequest {
  customer_id?: string
  country_id?: string
  service_tier?: ServiceTier
  notes?: string | null
}

export interface ReviewRequest {
  comment?: string
}

export interface QuotationListResponse {
  items: Quotation[]
  total: number
  page: number
  page_size: number
}

export interface QuotationListQuery {
  status?: QuotationStatus
  customer_id?: string
  page?: number
  page_size?: number
}

export interface QuotationHistoryEntry {
  from_status: QuotationStatus
  to_status: QuotationStatus
  actor_id?: string | null
  comment?: string | null
  at: string
}

export interface QuotationHistoryResponse {
  items: QuotationHistoryEntry[]
}

export type ExportFormat = 'pdf' | 'docx'
export type ExportLanguage = 'zh' | 'en' | 'bilingual'

export interface ExportFileDTO {
  id: string
  quotation_id: string
  format: ExportFormat
  language: ExportLanguage
  sha256: string
  file_size: number
  expires_at: string
  created_at: string
  download_url: string
}

export interface ExportRequest {
  format: ExportFormat
  language: ExportLanguage
}
