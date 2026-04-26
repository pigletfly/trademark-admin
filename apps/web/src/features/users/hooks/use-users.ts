import { queryOptions, keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { UserListQuery, UserListResponse } from '../types'

export const USERS_QUERY_KEY = ['admin', 'users'] as const

export const usersListQueryOptions = (query: UserListQuery) =>
  queryOptions({
    queryKey: [...USERS_QUERY_KEY, 'list', query] as const,
    queryFn: async (): Promise<UserListResponse> => {
      const res = await api.get<UserListResponse>('/admin/users', {
        params: {
          q: query.q || undefined,
          role: query.role || undefined,
          page: query.page ?? 1,
          page_size: query.page_size ?? 20,
        },
      })
      return res.data
    },
    placeholderData: keepPreviousData,
  })

export function useUsersList(query: UserListQuery) {
  return useQuery(usersListQueryOptions(query))
}
