import { type QueryClient } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'
import { type AuthUser } from '@/stores/auth-store'
import { meQueryOptions } from './hooks'

// Guard used in route `beforeLoad` to restrict access to specific roles.
// Assumes a parent route (e.g. `_authenticated`) has already ensured the user
// is authenticated; we just re-ensure the me cache and check the role code.
export async function requireRole(
  context: { queryClient: QueryClient },
  allowed: AuthUser['role'][]
) {
  const me = await context.queryClient.ensureQueryData(meQueryOptions)
  if (!allowed.includes(me.role)) {
    throw redirect({ to: '/403' })
  }
}
