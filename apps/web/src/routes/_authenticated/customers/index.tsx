import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { Customers } from '@/features/customers'
import { customersListQueryOptions } from '@/features/customers/hooks'

const searchSchema = z.object({
  q: z.string().optional().catch(''),
  page: z.number().int().min(1).optional().catch(1),
  page_size: z.number().int().min(1).max(100).optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/customers/')({
  validateSearch: searchSchema,
  loaderDeps: ({ search }) => ({ search }),
  loader: ({ context, deps }) =>
    context.queryClient.ensureQueryData(
      customersListQueryOptions({
        q: deps.search.q,
        page: deps.search.page,
        page_size: deps.search.page_size,
      })
    ),
  component: Customers,
})
