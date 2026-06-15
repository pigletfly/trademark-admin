import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { meQueryOptions } from '@/features/auth/hooks'
import { countriesQueryOptions } from '@/features/catalog/hooks'
import {
  madridPricingListQueryOptions,
  singleClassPricingListQueryOptions,
} from '@/features/pricing/hooks'
import { Pricing } from '@/features/pricing'

const searchSchema = z.object({
  country_id: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/pricing/')({
  validateSearch: searchSchema,
  beforeLoad: async ({ context }) => {
    const user = await context.queryClient.ensureQueryData(meQueryOptions)
    if (user.role !== 'reviewer' && user.role !== 'admin') {
      throw redirect({ to: '/403' })
    }
  },
  loaderDeps: ({ search }) => ({ search }),
  loader: async ({ context, deps }) => {
    await context.queryClient.ensureQueryData(countriesQueryOptions())
    await context.queryClient.ensureQueryData(
      singleClassPricingListQueryOptions({
        country_id: deps.search.country_id,
      })
    )
    await context.queryClient.ensureQueryData(
      madridPricingListQueryOptions({
        country_id: deps.search.country_id,
        include_base: true,
      })
    )
  },
  component: Pricing,
})
