import { createFileRoute } from '@tanstack/react-router'
import { Dashboard } from '@/features/dashboard'
import { dashboardSummaryQueryOptions } from '@/features/dashboard/hooks/use-dashboard'

export const Route = createFileRoute('/_authenticated/')({
  loader: ({ context }) =>
    context.queryClient.ensureQueryData(dashboardSummaryQueryOptions()),
  component: Dashboard,
})
