import { AxiosError } from 'axios'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  CreateOrReplaceMadridPricingRequest,
  CreateOrReplacePricingRequest,
  CreateOrReplaceSingleClassPricingRequest,
  MadridPricingEntry,
  PricingEntry,
  SingleClassPricingEntry,
} from '../types'
import { PRICING_QUERY_KEY } from './use-pricing-entries'

function mapPricingError(err: unknown): string {
  if (err instanceof AxiosError) {
    const code = (err.response?.data as { code?: string } | undefined)?.code
    if (code === 'ERR_INVALID_TIER') return '不支持的服务级别'
    if (code === 'ERR_ALREADY_DEPRECATED') return '该条目已被废止'
    if (err.response?.status === 403) return '没有权限修改定价'
  }
  return '保存失败，请稍后重试'
}

export function useCreateOrReplacePricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      body: CreateOrReplacePricingRequest
    ): Promise<PricingEntry> => {
      const res = await api.post<PricingEntry>('/pricing-entries', body)
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('定价已保存')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}

export function useDeprecatePricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (args: {
      id: string
      effective_to?: string
    }): Promise<PricingEntry> => {
      const res = await api.post<PricingEntry>(
        `/pricing-entries/${args.id}/deprecate`,
        {
          effective_to: args.effective_to,
        }
      )
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('定价已废止')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}

export function useCreateOrReplaceMadridPricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      body: CreateOrReplaceMadridPricingRequest
    ): Promise<MadridPricingEntry> => {
      const res = await api.post<MadridPricingEntry>(
        '/madrid-pricing-entries',
        body
      )
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('马德里定价已保存')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}

export function useCreateOrReplaceSingleClassPricing() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (
      body: CreateOrReplaceSingleClassPricingRequest
    ): Promise<SingleClassPricingEntry> => {
      const res = await api.post<SingleClassPricingEntry>(
        '/single-class-pricing-entries',
        body
      )
      return res.data
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: PRICING_QUERY_KEY })
      toast.success('单一分类定价已保存')
    },
    onError: (err) => {
      toast.error(mapPricingError(err))
    },
  })
}
