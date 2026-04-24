import { createFileRoute } from '@tanstack/react-router'
import { CustomerDetail } from '@/features/customers/detail'
import { customerDetailQueryOptions } from '@/features/customers/hooks'

export const Route = createFileRoute('/_authenticated/customers/$id')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(customerDetailQueryOptions(params.id)),
  component: CustomerDetail,
})
