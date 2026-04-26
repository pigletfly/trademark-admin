import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { requireRole } from '@/features/auth/guards'
import { Users } from '@/features/users'

const usersSearchSchema = z.object({
  q: z.string().optional().catch(''),
  role: z
    .enum(['salesperson', 'reviewer', 'admin'])
    .optional()
    .catch(undefined),
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/users/')({
  beforeLoad: async ({ context }) => {
    await requireRole(context, ['admin'])
  },
  validateSearch: usersSearchSchema,
  component: Users,
})
