import { useState } from 'react'
import {
  getCoreRowModel,
  flexRender,
  useReactTable,
} from '@tanstack/react-table'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { AuditLog } from '../types'
import { auditLogsColumns } from './audit-logs-columns'

interface Props {
  data: AuditLog[]
  total: number
  page: number
  pageSize: number
  resourceType: string
  userId: string
  onResourceTypeChange: (v: string) => void
  onUserIdChange: (v: string) => void
  onPageChange: (p: number) => void
}

const RESOURCE_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: '全部资源' },
  { value: 'user', label: '用户' },
  { value: 'customer', label: '客户' },
  { value: 'quotation', label: '报价' },
  { value: 'pricing_entry', label: '定价' },
  { value: 'country', label: '国家' },
]

export function AuditLogsTable({
  data,
  total,
  page,
  pageSize,
  resourceType,
  userId,
  onResourceTypeChange,
  onUserIdChange,
  onPageChange,
}: Props) {
  const [userDraft, setUserDraft] = useState(userId)
  const table = useReactTable({
    data,
    columns: auditLogsColumns,
    getCoreRowModel: getCoreRowModel(),
  })
  const pageCount = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className='flex flex-1 flex-col gap-4'>
      <form
        className='flex flex-wrap items-center gap-2'
        onSubmit={(e) => {
          e.preventDefault()
          onUserIdChange(userDraft.trim())
        }}
      >
        <Select
          value={resourceType || 'all'}
          onValueChange={(v) => onResourceTypeChange(v === 'all' ? '' : v)}
        >
          <SelectTrigger className='w-40'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RESOURCE_OPTIONS.map((o) => (
              <SelectItem key={o.value || 'all'} value={o.value || 'all'}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          placeholder='按操作人 UUID 过滤'
          className='max-w-xs'
          value={userDraft}
          onChange={(e) => setUserDraft(e.target.value)}
        />
        <Button type='submit' variant='secondary'>
          搜索
        </Button>
        {(resourceType || userId) && (
          <Button
            type='button'
            variant='ghost'
            onClick={() => {
              setUserDraft('')
              onResourceTypeChange('')
              onUserIdChange('')
            }}
          >
            清除
          </Button>
        )}
      </form>
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((h) => (
                  <TableHead key={h.id}>
                    {h.isPlaceholder
                      ? null
                      : flexRender(h.column.columnDef.header, h.getContext())}
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
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={auditLogsColumns.length}
                  className='h-24 text-center'
                >
                  暂无审计记录
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
