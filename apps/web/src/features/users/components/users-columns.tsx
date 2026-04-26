import { type ColumnDef } from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import type { User, UserRole, UserStatus } from '../types'
import { UserRowActions } from './user-row-actions'

const roleLabels: Record<UserRole, string> = {
  salesperson: '业务员',
  reviewer: '国际部商务',
  admin: '管理员',
}

const statusLabels: Record<UserStatus, string> = {
  active: '启用',
  disabled: '禁用',
}

export const usersColumns: ColumnDef<User>[] = [
  {
    accessorKey: 'name',
    header: '姓名',
    cell: ({ row }) => (
      <span className='font-medium'>{row.original.name}</span>
    ),
  },
  {
    accessorKey: 'email',
    header: '邮箱',
  },
  {
    accessorKey: 'phone',
    header: '电话',
    cell: ({ getValue }) => getValue<string | undefined>() || '—',
  },
  {
    accessorKey: 'role',
    header: '角色',
    cell: ({ getValue }) => (
      <Badge variant='secondary'>{roleLabels[getValue<UserRole>()]}</Badge>
    ),
  },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ getValue }) => {
      const s = getValue<UserStatus>()
      return (
        <Badge variant={s === 'active' ? 'default' : 'outline'}>
          {statusLabels[s]}
        </Badge>
      )
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => <UserRowActions user={row.original} />,
  },
]
