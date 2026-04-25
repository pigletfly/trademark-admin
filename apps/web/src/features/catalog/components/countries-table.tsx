import { useMemo, useState } from 'react'
import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Country } from '../types'

interface Props {
  data: Country[]
  onEdit: (row: Country) => void
}

export function CountriesTable({ data, onEdit }: Props) {
  const [filter, setFilter] = useState('')

  const columns = useMemo<ColumnDef<Country>[]>(
    () => [
      { accessorKey: 'code', header: 'ISO' },
      { accessorKey: 'name_zh', header: '中文名' },
      { accessorKey: 'name_en', header: '英文名' },
      {
        accessorKey: 'is_madrid_member',
        header: 'Madrid',
        cell: ({ getValue }) =>
          getValue<boolean>() ? <Badge>成员</Badge> : <span className='text-muted-foreground'>—</span>,
      },
      {
        accessorKey: 'default_acceptance_days',
        header: '受理天数',
        cell: ({ getValue }) => getValue<number | null>() ?? '—',
      },
      {
        accessorKey: 'default_registration_months',
        header: '注册月数',
        cell: ({ getValue }) => getValue<number | null>() ?? '—',
      },
      {
        accessorKey: 'enabled',
        header: '启用',
        cell: ({ getValue }) =>
          getValue<boolean>() ? <Badge variant='secondary'>启用</Badge> : <Badge variant='outline'>停用</Badge>,
      },
      {
        id: 'actions',
        header: '',
        cell: ({ row }) => (
          <Button size='sm' variant='ghost' onClick={() => onEdit(row.original)}>
            编辑
          </Button>
        ),
      },
    ],
    [onEdit]
  )

  const table = useReactTable({
    data,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    globalFilterFn: (row, _col, val: string) => {
      const v = val.toLowerCase()
      const r = row.original
      return [r.code, r.name_zh, r.name_en].some((f) => f.toLowerCase().includes(v))
    },
  })

  return (
    <div className='flex flex-col gap-3'>
      <Input
        placeholder='按 ISO / 中文名 / 英文名搜索'
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className='max-w-xs'
      />
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => (
                  <TableHead key={h.id}>
                    {h.isPlaceholder ? null : flexRender(h.column.columnDef.header, h.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className='h-24 text-center'>
                  无数据
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
