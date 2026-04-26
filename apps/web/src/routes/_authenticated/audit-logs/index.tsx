import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { requireRole } from '@/features/auth/guards'
import { AuditLogs } from '@/features/audit-logs'

const auditLogsSearchSchema = z.object({
  resource_type: z.string().optional().catch(''),
  user_id: z.string().optional().catch(''),
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(20),
})

export const Route = createFileRoute('/_authenticated/audit-logs/')({
  beforeLoad: async ({ context }) => {
    await requireRole(context, ['admin'])
  },
  validateSearch: auditLogsSearchSchema,
  component: AuditLogs,
})
