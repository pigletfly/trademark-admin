import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { QUOTATION_STATUS_LABEL_ZH, type QuotationStatus } from '../types'

const VARIANT: Record<QuotationStatus, string> = {
  draft: 'bg-muted text-muted-foreground',
  submitted: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200',
  approved: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200',
  rejected: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200',
  cancelled: 'bg-muted text-muted-foreground opacity-70',
}

export function QuotationStatusBadge({ status }: { status: QuotationStatus }) {
  return (
    <Badge variant='outline' className={cn('rounded px-2 py-0.5 text-xs font-medium', VARIANT[status])}>
      {QUOTATION_STATUS_LABEL_ZH[status]}
    </Badge>
  )
}
