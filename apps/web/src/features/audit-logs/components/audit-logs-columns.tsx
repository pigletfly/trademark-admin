import { type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import type { AuditLog } from '../types'

function formatTs(iso: string) {
  const d = new Date(iso)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export const auditLogsColumns: ColumnDef<AuditLog>[] = [
  {
    accessorKey: 'created_at',
    header: '时间',
    cell: ({ getValue }) => (
      <span className='font-mono text-xs'>{formatTs(getValue<string>())}</span>
    ),
  },
  {
    accessorKey: 'action',
    header: '动作',
    cell: ({ getValue }) => (
      <Badge variant='secondary'>{getValue<string>()}</Badge>
    ),
  },
  {
    accessorKey: 'resource_type',
    header: '资源类型',
  },
  {
    accessorKey: 'resource_id',
    header: '资源 ID',
    cell: ({ getValue }) => (
      <span className='font-mono text-xs'>{getValue<string>()}</span>
    ),
  },
  {
    accessorKey: 'user_id',
    header: '操作人',
    cell: ({ getValue }) => {
      const v = getValue<string | undefined>()
      return v ? (
        <span className='font-mono text-xs'>{v.slice(0, 8)}…</span>
      ) : (
        <span className='text-muted-foreground'>系统</span>
      )
    },
  },
  {
    accessorKey: 'ip',
    header: 'IP',
    cell: ({ getValue }) => getValue<string | undefined>() || '—',
  },
  {
    accessorKey: 'changes_json',
    header: '变更',
    cell: ({ getValue }) => {
      const v = getValue<Record<string, unknown> | null | undefined>()
      if (!v) return <span className='text-muted-foreground'>—</span>
      return (
        <details className='max-w-xs'>
          <summary className='cursor-pointer text-xs text-primary'>查看</summary>
          <pre className='mt-1 overflow-auto rounded bg-muted p-2 text-[10px] leading-tight'>
            {JSON.stringify(v, null, 2)}
          </pre>
        </details>
      )
    },
  },
]
