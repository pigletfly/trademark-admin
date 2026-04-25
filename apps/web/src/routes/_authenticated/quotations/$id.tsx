import { createFileRoute } from '@tanstack/react-router'
import { QuotationDetail } from '@/features/quotation/detail'
import {
  quotationDetailQueryOptions,
  quotationHistoryQueryOptions,
} from '@/features/quotation/hooks'

export const Route = createFileRoute('/_authenticated/quotations/$id')({
  loader: async ({ context, params }) => {
    await Promise.all([
      context.queryClient.ensureQueryData(quotationDetailQueryOptions(params.id)),
      context.queryClient.ensureQueryData(quotationHistoryQueryOptions(params.id)),
    ])
  },
  component: QuotationDetail,
})
