import { useState } from 'react'
import { getCoreRowModel, flexRender, useReactTable } from '@tanstack/react-table'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Customer } from '../types'
import { customersColumns } from './customers-columns'

interface Props {
  data: Customer[]
  total: number
  page: number
  pageSize: number
  queryText: string
  onQueryChange: (q: string) => void
  onPageChange: (page: number) => void
}

export function CustomersTable({
  data,
  total,
  page,
  pageSize,
  queryText,
  onQueryChange,
  onPageChange,
}: Props) {
  const [draft, setDraft] = useState(queryText)
  const table = useReactTable({
    data,
    columns: customersColumns,
    getCoreRowModel: getCoreRowModel(),
  })

  const pageCount = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className='flex flex-1 flex-col gap-4'>
      <form
        className='flex items-center gap-2'
        onSubmit={(e) => {
          e.preventDefault()
          onQueryChange(draft.trim())
        }}
      >
        <Input
          placeholder='按姓名或行业搜索'
          className='max-w-xs'
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <Button type='submit' variant='secondary'>
          搜索
        </Button>
        {queryText && (
          <Button
            type='button'
            variant='ghost'
            onClick={() => {
              setDraft('')
              onQueryChange('')
            }}
          >
            清除
          </Button>
        )}
      </form>
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
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
                <TableCell colSpan={customersColumns.length} className='h-24 text-center'>
                  暂无客户
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className='flex items-center justify-between'>
        <span className='text-sm text-muted-foreground'>
          共 {total} 条，第 {page} / {pageCount} 页
        </span>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
          >
            上一页
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={page >= pageCount}
            onClick={() => onPageChange(page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  )
}
