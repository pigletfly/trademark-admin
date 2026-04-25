import { createFileRoute } from '@tanstack/react-router'
import { QuotationPrint } from '@/features/quotation/print'
import { quotationDetailQueryOptions } from '@/features/quotation/hooks'

export const Route = createFileRoute('/_authenticated/quotations/$id/print')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(quotationDetailQueryOptions(params.id)),
  component: QuotationPrint,
})
