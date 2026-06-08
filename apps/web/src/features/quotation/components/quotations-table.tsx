import {
  flexRender,
  getCoreRowModel,
  type ColumnDef,
  useReactTable,
} from '@tanstack/react-table'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Quotation } from '../types'

interface Props {
  data: Quotation[]
  columns: ColumnDef<Quotation>[]
  isLoading?: boolean
}

export function QuotationsTable({ data, columns, isLoading }: Props) {
  const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel() })
  return (
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
          {isLoading && (
            <TableRow>
              <TableCell colSpan={columns.length} className='text-center text-sm text-muted-foreground'>
                加载中…
              </TableCell>
            </TableRow>
          )}
          {!isLoading && table.getRowModel().rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={columns.length} className='text-center text-sm text-muted-foreground'>
                暂无报价记录
              </TableCell>
            </TableRow>
          )}
          {!isLoading &&
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((c) => (
                  <TableCell key={c.id}>{flexRender(c.column.columnDef.cell, c.getContext())}</TableCell>
                ))}
              </TableRow>
            ))}
        </TableBody>
      </Table>
    </div>
  )
}
