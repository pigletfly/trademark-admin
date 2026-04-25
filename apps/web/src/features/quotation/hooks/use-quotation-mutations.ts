import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  CreateQuotationRequest,
  Quotation,
  ReviewRequest,
  UpdateDraftRequest,
} from '../types'
import { QUOTATIONS_QUERY_KEY } from './use-quotations'

function mapQuotationError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_INVALID_TIER') return '不支持的服务级别'
    if (code === 'ERR_INVALID_TRANSITION') return '当前状态不允许该操作'
    if (code === 'ERR_MISSING_PRICING') return '该国家/级别暂无定价，请联系管理员'
    if (code === 'ERR_NOT_OWNER') return '只能操作自己创建的报价'
    if (err.response?.status === 403) return '没有权限执行该操作'
    if (err.response?.status === 404) return '报价不存在'
  }
  return '请求失败，请稍后重试'
}

function invalidate(qc: ReturnType<typeof useQueryClient>, id?: string) {
  void qc.invalidateQueries({ queryKey: QUOTATIONS_QUERY_KEY })
  if (id) {
    void qc.invalidateQueries({ queryKey: [...QUOTATIONS_QUERY_KEY, 'detail', id] as const })
    void qc.invalidateQueries({ queryKey: [...QUOTATIONS_QUERY_KEY, 'history', id] as const })
  }
}

export function useCreateQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateQuotationRequest): Promise<Quotation> => {
      const res = await api.post<Quotation>('/quotations', body)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价草稿已创建')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useUpdateQuotationDraft() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; body: UpdateDraftRequest }): Promise<Quotation> => {
      const res = await api.patch<Quotation>(`/quotations/${args.id}`, args.body)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价草稿已保存')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useSubmitQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string): Promise<Quotation> => {
      const res = await api.post<Quotation>(`/quotations/${id}/submit`)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价已提交待审核')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useReviewQuotation(approve: boolean) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; comment?: string }): Promise<Quotation> => {
      const path = approve ? 'approve' : 'reject'
      const body: ReviewRequest = args.comment ? { comment: args.comment } : {}
      const res = await api.post<Quotation>(`/quotations/${args.id}/${path}`, body)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success(approve ? '报价已通过' : '报价已驳回')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useCancelQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; comment?: string }): Promise<Quotation> => {
      const body: ReviewRequest = args.comment ? { comment: args.comment } : {}
      const res = await api.post<Quotation>(`/quotations/${args.id}/cancel`, body)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价已取消')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}
