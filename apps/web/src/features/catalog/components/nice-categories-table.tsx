import { useState } from 'react'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { NiceCategory } from '../types'

interface Props {
  data: NiceCategory[]
}

export function NiceCategoriesTable({ data }: Props) {
  const [filter, setFilter] = useState('')

  const filtered = data.filter((r) => {
    if (!filter.trim()) return true
    const f = filter.toLowerCase()
    return (
      String(r.code).includes(f) ||
      r.name_zh.toLowerCase().includes(f) ||
      r.name_en.toLowerCase().includes(f)
    )
  })

  return (
    <div className='flex flex-col gap-3'>
      <Input
        placeholder='按代码或名称搜索'
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className='max-w-xs'
      />
      <div className='overflow-hidden rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-20'>代码</TableHead>
              <TableHead>中文名</TableHead>
              <TableHead>英文名</TableHead>
              <TableHead>中文描述</TableHead>
              <TableHead>英文描述</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length ? (
              filtered.map((r) => (
                <TableRow key={r.code}>
                  <TableCell className='font-mono'>{r.code}</TableCell>
                  <TableCell>{r.name_zh}</TableCell>
                  <TableCell>{r.name_en}</TableCell>
                  <TableCell className='max-w-xs truncate' title={r.description_zh ?? ''}>
                    {r.description_zh ?? '—'}
                  </TableCell>
                  <TableCell className='max-w-xs truncate' title={r.description_en ?? ''}>
                    {r.description_en ?? '—'}
                  </TableCell>
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={5} className='h-24 text-center'>
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
