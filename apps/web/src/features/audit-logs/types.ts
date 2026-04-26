export interface AuditLog {
  id: string
  user_id?: string
  action: string
  resource_type: string
  resource_id: string
  changes_json?: Record<string, unknown> | null
  ip?: string
  user_agent?: string
  created_at: string
}

export interface AuditLogListQuery {
  resource_type?: string
  user_id?: string
  from?: string
  to?: string
  page?: number
  page_size?: number
}

export interface AuditLogListResponse {
  items: AuditLog[]
  page: number
  page_size: number
  total: number
}
