import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQueryOptions } from '@/features/auth/hooks'
import { niceCategoriesQueryOptions } from '@/features/catalog/hooks'
import { CatalogNiceCategories } from '@/features/catalog/nice-categories'

export const Route = createFileRoute('/_authenticated/catalog/nice-categories')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loader: ({ context }) =>
    context.queryClient.ensureQueryData(niceCategoriesQueryOptions),
  component: CatalogNiceCategories,
})
