import type { ColumnDef } from '@tanstack/react-table'
import { Link } from '@tanstack/react-router'
import type { Quotation } from '../types'
import { QuotationStatusBadge } from './quotation-status-badge'

function formatCNY(cents: number | null | undefined): string {
  if (cents == null) return '—'
  return '¥' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export const quotationColumns: ColumnDef<Quotation>[] = [
  {
    accessorKey: 'id',
    header: '编号',
    cell: ({ row }) => (
      <Link
        to='/quotations/$id'
        params={{ id: row.original.id }}
        className='text-sm font-mono text-primary underline-offset-2 hover:underline'
      >
        {row.original.id.slice(0, 8)}
      </Link>
    ),
  },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ row }) => <QuotationStatusBadge status={row.original.status} />,
  },
  {
    accessorKey: 'service_tier',
    header: '级别',
  },
  {
    accessorKey: 'total_cny_cents',
    header: '金额',
    cell: ({ row }) => (
      <span className='font-medium'>{formatCNY(row.original.total_cny_cents)}</span>
    ),
  },
  {
    accessorKey: 'created_at',
    header: '创建时间',
    cell: ({ row }) => new Date(row.original.created_at).toLocaleString(),
  },
  {
    accessorKey: 'submitted_at',
    header: '提交时间',
    cell: ({ row }) =>
      row.original.submitted_at ? new Date(row.original.submitted_at).toLocaleString() : '—',
  },
]
