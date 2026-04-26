import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  CreateUserRequest,
  ResetPasswordResponse,
  UpdateUserRequest,
  User,
} from '../types'
import { USERS_QUERY_KEY } from './use-users'

function mapUserError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_EMAIL_TAKEN') return '该邮箱已被使用'
    if (code === 'ERR_NOT_FOUND') return '用户不存在'
    if (code === 'ERR_SELF_PROTECTED') return '不能修改自己的角色/状态或重置自己的密码'
    if (err.response?.status === 403) return '没有权限操作用户'
  }
  return '请求失败，请稍后重试'
}

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateUserRequest): Promise<User> => {
      const res = await api.post<{ user: User }>('/admin/users', body)
      return res.data.user
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: USERS_QUERY_KEY })
      toast.success('用户已创建')
    },
    onError: (err) => {
      toast.error(mapUserError(err))
    },
  })
}

export function useUpdateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      body: UpdateUserRequest
    }): Promise<User> => {
      const res = await api.patch<{ user: User }>(
        `/admin/users/${args.id}`,
        args.body
      )
      return res.data.user
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: USERS_QUERY_KEY })
      toast.success('用户已更新')
    },
    onError: (err) => {
      toast.error(mapUserError(err))
    },
  })
}

export function useResetUserPassword() {
  return useMutation({
    mutationFn: async (id: string): Promise<ResetPasswordResponse> => {
      const res = await api.post<ResetPasswordResponse>(
        `/admin/users/${id}/reset-password`
      )
      return res.data
    },
    onError: (err) => {
      toast.error(mapUserError(err))
    },
  })
}
