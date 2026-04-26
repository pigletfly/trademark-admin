import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  AdjustRequest,
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
    if (code === 'ERR_EMPTY_ADJUST') return '请至少输入一行报价项'
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

export function useWithdrawQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string): Promise<Quotation> => {
      const res = await api.post<Quotation>(`/quotations/${id}/withdraw`)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价已撤回为草稿')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useCopyQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (id: string): Promise<Quotation> => {
      const res = await api.post<Quotation>(`/quotations/${id}/copy`)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('报价已复制为新草稿')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

export function useAdjustQuotation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: { id: string; body: AdjustRequest }): Promise<Quotation> => {
      const res = await api.post<Quotation>(`/quotations/${args.id}/adjust`, args.body)
      return res.data
    },
    onSuccess: (q) => {
      invalidate(qc, q.id)
      toast.success('调价已保存')
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

/**
 * useCreateAndSubmit runs POST /quotations then POST /quotations/:id/submit.
 *
 * Failure semantics:
 * - If create fails: throws; caller shows toast and draft stays in localStorage
 *   so the user can retry.
 * - If create succeeds but submit fails: resolves with { id, submitted: false }.
 *   The caller should clear localStorage (draft exists on the server) and
 *   surface a toast like "草稿已创建,但提交失败,请在详情页重试".
 */
export function useCreateAndSubmit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      body: CreateQuotationRequest,
    ): Promise<{ id: string; submitted: boolean }> => {
      const created = await api.post<Quotation>('/quotations', body)
      try {
        await api.post<Quotation>(`/quotations/${created.data.id}/submit`)
        return { id: created.data.id, submitted: true }
      } catch {
        return { id: created.data.id, submitted: false }
      }
    },
    onSuccess: (result) => {
      invalidate(qc, result.id)
      if (result.submitted) {
        toast.success('报价已提交待审核')
      } else {
        toast.warning('草稿已创建,但提交失败,请在详情页重试')
      }
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}

/**
 * useUpdateAndSubmit runs PATCH /quotations/:id then POST /quotations/:id/submit.
 * Same failure semantics as useCreateAndSubmit.
 */
export function useUpdateAndSubmit() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      body: UpdateDraftRequest
    }): Promise<{ id: string; submitted: boolean }> => {
      await api.patch<Quotation>(`/quotations/${args.id}`, args.body)
      try {
        await api.post<Quotation>(`/quotations/${args.id}/submit`)
        return { id: args.id, submitted: true }
      } catch {
        return { id: args.id, submitted: false }
      }
    },
    onSuccess: (result) => {
      invalidate(qc, result.id)
      if (result.submitted) {
        toast.success('报价已提交待审核')
      } else {
        toast.warning('草稿已更新,但提交失败,请在详情页重试')
      }
    },
    onError: (err) => toast.error(mapQuotationError(err)),
  })
}
