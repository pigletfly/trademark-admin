import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { DashboardSummary } from '../types'

export const DASHBOARD_QUERY_KEY = ['dashboard', 'summary'] as const

export const dashboardSummaryQueryOptions = () =>
  queryOptions({
    queryKey: DASHBOARD_QUERY_KEY,
    queryFn: async (): Promise<DashboardSummary> => {
      const res = await api.get<DashboardSummary>('/dashboard/summary')
      return res.data
    },
    staleTime: 60 * 1000, // 1 min — dashboard numbers don't need to be live
  })

export function useDashboardSummary() {
  return useQuery(dashboardSummaryQueryOptions())
}
