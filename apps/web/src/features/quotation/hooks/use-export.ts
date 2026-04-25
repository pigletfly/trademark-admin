import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'

import { api } from '@/lib/api'

import type { ExportFileDTO, ExportRequest } from '../types'

export function useExportQuotation(quotationId: string) {
  return useMutation({
    mutationFn: async (req: ExportRequest) => {
      const { data } = await api.post<ExportFileDTO>(
        `/quotations/${quotationId}/export`,
        req,
      )
      return data
    },
    onSuccess: (data) => {
      window.open(data.download_url, '_blank', 'noopener')
      toast.success(
        data.format === 'pdf'
          ? '已生成 PDF / PDF ready'
          : '已生成 Word / Word ready',
      )
    },
    onError: (err: unknown) => {
      toast.error(`导出失败 Export failed: ${String(err)}`)
    },
  })
}
