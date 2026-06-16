import type { ColumnDef } from '@tanstack/react-table'
import { Link } from '@tanstack/react-router'
import type { Quotation, ServiceTier } from '../types'
import { QuotationStatusBadge } from './quotation-status-badge'

function formatCNY(cents: number | null | undefined): string {
  if (cents == null) return '—'
  return '¥' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

// 列表「代理级别」与新建向导口径保持一致：service_tier 由 agent_level 派生
// (A 代理 → basic、B 代理 → standard)；premium 属历史数据，并入 B 代理显示。
const AGENT_LEVEL_LABELS: Record<ServiceTier, string> = {
  basic: 'A 代理',
  standard: 'B 代理',
  premium: 'B 代理',
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
    header: '代理级别',
    cell: ({ row }) => AGENT_LEVEL_LABELS[row.original.service_tier],
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
