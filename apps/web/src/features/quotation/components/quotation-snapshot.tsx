import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { QuotationSnapshot } from '../types'
import { translateFeeItemLabel } from '../fee-item-label'

function formatCNY(cents: number): string {
  return (
    '¥' +
    (cents / 100).toLocaleString('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  )
}

export function QuotationSnapshotView({
  snapshot,
}: {
  snapshot: QuotationSnapshot
}) {
  return (
    <div className='space-y-2'>
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>费用项</TableHead>
              <TableHead className='text-right'>金额（人民币）</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {snapshot.lines.map((l, index) => (
              <TableRow key={`${l.fee_item}-${index}`}>
                <TableCell className='font-mono text-sm'>
                  {translateFeeItemLabel(l.fee_item)}
                </TableCell>
                <TableCell className='text-right font-medium'>
                  {formatCNY(l.amount_cny_cents)}
                </TableCell>
              </TableRow>
            ))}
            <TableRow>
              <TableCell className='font-semibold'>总计</TableCell>
              <TableCell className='text-right text-lg font-bold'>
                {formatCNY(snapshot.total_cny_cents)}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <p className='text-xs text-muted-foreground break-all'>
        签名：<code>{snapshot.signature}</code>
      </p>
    </div>
  )
}
