import { Link } from '@tanstack/react-router'
import { type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import type { Customer } from '../types'

export const customersColumns: ColumnDef<Customer>[] = [
  {
    accessorKey: 'name',
    header: '客户名称',
    cell: ({ row }) => (
      <Link
        to='/customers/$id'
        params={{ id: row.original.id }}
        className='font-medium text-primary underline-offset-4 hover:underline'
      >
        {row.original.name}
      </Link>
    ),
  },
  {
    accessorKey: 'industry',
    header: '行业',
    cell: ({ getValue }) => (getValue<string | null>() ?? '—'),
  },
  {
    accessorKey: 'is_returning',
    header: '回头客户',
    cell: ({ getValue }) =>
      getValue<boolean>() ? <Badge>回头</Badge> : <span className='text-muted-foreground'>—</span>,
  },
  {
    accessorKey: 'price_sensitive',
    header: '价格敏感',
    cell: ({ getValue }) =>
      getValue<boolean>() ? <Badge variant='secondary'>敏感</Badge> : <span className='text-muted-foreground'>—</span>,
  },
  {
    accessorKey: 'contact_name',
    header: '联系人',
    cell: ({ getValue }) => getValue<string | null>() ?? '—',
  },
  {
    accessorKey: 'contact_phone',
    header: '电话',
    cell: ({ getValue }) => getValue<string | null>() ?? '—',
  },
]
