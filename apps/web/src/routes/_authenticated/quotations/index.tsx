import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Quotations } from '@/features/quotation'
import { quotationsListQueryOptions } from '@/features/quotation/hooks'

const searchSchema = z.object({
  status: z.enum(['draft', 'submitted', 'approved', 'rejected', 'cancelled']).optional().catch(undefined),
  page: z.number().int().min(1).optional().catch(1),
  page_size: z.number().int().min(1).max(100).optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/quotations/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, deps }) =>
    context.queryClient.ensureQueryData(
      quotationsListQueryOptions({
        status: deps.search.status,
        page: deps.search.page,
        page_size: deps.search.page_size,
      })
    ),
  component: Quotations,
})
