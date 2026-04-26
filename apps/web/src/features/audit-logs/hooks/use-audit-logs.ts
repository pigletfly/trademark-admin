import { keepPreviousData, queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { AuditLogListQuery, AuditLogListResponse } from '../types'

export const AUDIT_LOGS_QUERY_KEY = ['admin', 'audit-logs'] as const

export const auditLogsListQueryOptions = (query: AuditLogListQuery) =>
  queryOptions({
    queryKey: [...AUDIT_LOGS_QUERY_KEY, 'list', query] as const,
    queryFn: async (): Promise<AuditLogListResponse> => {
      const res = await api.get<AuditLogListResponse>('/admin/audit-logs', {
        params: {
          resource_type: query.resource_type || undefined,
          user_id: query.user_id || undefined,
          from: query.from || undefined,
          to: query.to || undefined,
          page: query.page ?? 1,
          page_size: query.page_size ?? 20,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export function useAuditLogsList(query: AuditLogListQuery) {
  return useQuery(auditLogsListQueryOptions(query))
}
