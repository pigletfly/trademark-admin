import { createFileRoute, redirect } from '@tanstack/react-router'
import { meQueryOptions } from '@/features/auth/hooks'
import { countriesQueryOptions } from '@/features/catalog/hooks'
import { CatalogCountries } from '@/features/catalog/countries'

export const Route = createFileRoute('/_authenticated/catalog/countries')({
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loader: ({ context }) =>
    context.queryClient.ensureQueryData(countriesQueryOptions(true)),
  component: CatalogCountries,
})
