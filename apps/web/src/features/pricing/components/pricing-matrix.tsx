import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  SERVICE_TIERS,
  SERVICE_TIER_LABEL_ZH,
  type PricingEntry,
  type ServiceTier,
} from '../types'

export interface MatrixCell {
  feeItem: string
  byTier: Partial<Record<ServiceTier, PricingEntry>>
}

// toMatrix pivots a flat list of active PricingEntries into rows by
// fee_item, columns by service_tier. Undefined cells mean no active
// entry for that dimension — render as "—".
export function toMatrix(entries: PricingEntry[]): MatrixCell[] {
  const byFeeItem = new Map<string, MatrixCell>()
  for (const e of entries) {
    if (e.effective_to) continue // defensive: backend should already filter
    let cell = byFeeItem.get(e.fee_item)
    if (!cell) {
      cell = { feeItem: e.fee_item, byTier: {} }
      byFeeItem.set(e.fee_item, cell)
    }
    cell.byTier[e.service_tier] = e
  }
  return Array.from(byFeeItem.values()).sort((a, b) =>
    a.feeItem.localeCompare(b.feeItem)
  )
}

export function formatCNY(cents: number): string {
  const yuan = cents / 100
  return '¥' + yuan.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

interface Props {
  entries: PricingEntry[]
  canEdit: boolean
  onEditCell: (feeItem: string, tier: ServiceTier, current?: PricingEntry) => void
  onOpenHistory: (feeItem: string, tier: ServiceTier) => void
}

export function PricingMatrix({ entries, canEdit, onEditCell, onOpenHistory }: Props) {
  const matrix = useMemo(() => toMatrix(entries), [entries])

  if (matrix.length === 0) {
    return <p className='text-sm text-muted-foreground'>该国家暂无定价条目。{canEdit && ' 点击右上方“新增条目”开始添加。'}</p>
  }

  return (
    <div className='overflow-hidden rounded-md border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='w-64'>费用项</TableHead>
            {SERVICE_TIERS.map((t) => (
              <TableHead key={t}>{SERVICE_TIER_LABEL_ZH[t]}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {matrix.map((row) => (
            <TableRow key={row.feeItem}>
              <TableCell className='font-mono text-sm'>{row.feeItem}</TableCell>
              {SERVICE_TIERS.map((t) => {
                const entry = row.byTier[t]
                return (
                  <TableCell key={t}>
                    <div className='flex items-center gap-2'>
                      <span className={entry ? 'font-medium' : 'text-muted-foreground'}>
                        {entry ? formatCNY(entry.amount_cny_cents) : '—'}
                      </span>
                      {entry && (
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => onOpenHistory(row.feeItem, t)}
                          className='h-6 px-2 text-xs'
                        >
                          历史
                        </Button>
                      )}
                      {canEdit && (
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => onEditCell(row.feeItem, t, entry)}
                          className='h-6 px-2 text-xs'
                        >
                          {entry ? '修改' : '新增'}
                        </Button>
                      )}
                    </div>
                  </TableCell>
                )
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
