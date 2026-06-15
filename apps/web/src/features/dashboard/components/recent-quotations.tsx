import { Link } from '@tanstack/react-router'
import { QuotationStatusBadge } from '@/features/quotation/components/quotation-status-badge'
import { SERVICE_TIER_LABEL_ZH } from '@/features/pricing/types'
import type { RecentQuotation } from '../types'

function formatCNY(cents: number | null | undefined): string {
  if (cents == null) return '—'
  return '¥' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export function RecentQuotations({ items }: { items: RecentQuotation[] }) {
  if (items.length === 0) {
    return <p className='text-sm text-muted-foreground'>暂无报价记录</p>
  }
  return (
    <ul className='divide-y'>
      {items.map((q) => (
        <li key={q.id} className='flex items-center justify-between py-2 text-sm'>
          <div className='flex items-center gap-2'>
            <Link
              to='/quotations/$id'
              params={{ id: q.id }}
              className='font-mono text-primary underline-offset-2 hover:underline'
            >
              {q.id.slice(0, 8)}
            </Link>
            <QuotationStatusBadge status={q.status} />
            <span className='text-xs text-muted-foreground'>
              {SERVICE_TIER_LABEL_ZH[q.service_tier] ?? q.service_tier}
            </span>
          </div>
          <div className='flex items-center gap-3'>
            <span className='font-medium'>{formatCNY(q.total_cny_cents)}</span>
            <span className='text-xs text-muted-foreground'>
              {new Date(q.updated_at).toLocaleDateString('zh-CN')}
            </span>
          </div>
        </li>
      ))}
    </ul>
  )
}
