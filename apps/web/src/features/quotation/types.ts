// Mirrors the Go DTOs in apps/api/internal/quotation/dto.go.
export type QuotationStatus =
  | 'draft'
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'cancelled'

export const QUOTATION_STATUS_LABEL_ZH: Record<QuotationStatus, string> = {
  draft: '草稿',
  submitted: '已提交',
  approved: '已通过',
  rejected: '已驳回',
  cancelled: '已取消',
}

export type ServiceTier = 'basic' | 'standard' | 'premium'

export type RegistrationMethod = 'madrid' | 'single'
export type AgentLevel = 'agent_a' | 'agent_b'
export type QuoteInfoSection =
  | 'acceptance_time'
  | 'registration_time'
  | 'required_documents'
  | 'registration_method_intro'
  | 'real_cases'

export interface SnapshotLine {
  fee_item: string
  amount_cny_cents: number
  /**
   * The pricing_entries.id that this line was derived from. null/undefined
   * means either a legacy snapshot (created before M4) or a reviewer-adjusted
   * line ("orphan" manual override). Set when the line came from
   * pricing.Calculate.
   */
  source_pricing_entry_id?: string | null
  source_pricing_table?: string
  source_pricing_id?: string | null
  registration_method?: RegistrationMethod
  country_id?: string | null
  country_area?: string
  quantity?: number
  unit_amount_cny_cents?: number | null
  official_fee_chf_cents?: number | null
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
  country_ids?: string[]
  nice_category_codes?: number[]
  registration_methods?: RegistrationMethod[]
  agent_level?: AgentLevel
  service_tier: ServiceTier
  status: QuotationStatus
  snapshot?: QuotationSnapshot
  total_cny_cents?: number | null
  signature?: string | null
  serial_no?: string | null
  submitted_at?: string | null
  reviewed_at?: string | null
  reviewed_by?: string | null
  review_comment?: string | null
  info_sections?: QuoteInfoSection[]
  notes?: string | null
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateQuotationRequest {
  customer_id: string
  country_id: string
  country_ids?: string[]
  nice_category_codes?: number[]
  registration_methods?: RegistrationMethod[]
  agent_level?: AgentLevel
  service_tier: ServiceTier
  info_sections?: QuoteInfoSection[]
  notes?: string | null
}

export interface UpdateDraftRequest {
  customer_id?: string
  country_id?: string
  country_ids?: string[]
  nice_category_codes?: number[]
  registration_methods?: RegistrationMethod[]
  agent_level?: AgentLevel
  service_tier?: ServiceTier
  info_sections?: QuoteInfoSection[]
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
  diff_json?: SnapshotDiff | null
}

export interface QuotationHistoryResponse {
  items: QuotationHistoryEntry[]
}

export type ExportFormat = 'pdf' | 'docx' | 'xlsx'
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

export interface SnapshotLineDelta {
  fee_item: string
  before: number
  after: number
}

export interface SnapshotDiff {
  lines_added?: SnapshotLineDelta[]
  lines_removed?: SnapshotLineDelta[]
  lines_updated?: SnapshotLineDelta[]
  total_before: number
  total_after: number
}

export interface AdjustRequest {
  lines: SnapshotLine[]
  comment?: string
}
