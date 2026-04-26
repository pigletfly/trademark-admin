import { useState } from 'react'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Plus, Trash } from 'lucide-react'

import { useAdjustQuotation } from '../hooks/use-quotation-mutations'
import type { Quotation, SnapshotLine } from '../types'

interface Props {
  quotation: Quotation
  trigger: React.ReactNode
}

export function QuotationAdjustSheet({ quotation, trigger }: Props) {
  const [open, setOpen] = useState(false)
  const [lines, setLines] = useState<SnapshotLine[]>(
    () => (quotation.snapshot?.lines ?? []).map((l) => ({ ...l }))
  )
  const [comment, setComment] = useState('')
  const adjustMut = useAdjustQuotation()

  const total = lines.reduce((sum, l) => sum + (l.amount_cny_cents || 0), 0)
  const hasValidLine = lines.some((l) => l.fee_item.trim().length > 0)

  function handleOpenChange(next: boolean) {
    setOpen(next)
    if (next) {
      setLines((quotation.snapshot?.lines ?? []).map((l) => ({ ...l })))
      setComment('')
    }
  }

  function updateLine(idx: number, patch: Partial<SnapshotLine>) {
    setLines((prev) => prev.map((l, i) => (i === idx ? { ...l, ...patch } : l)))
  }

  function removeLine(idx: number) {
    setLines((prev) => prev.filter((_, i) => i !== idx))
  }

  function addLine() {
    setLines((prev) => [...prev, { fee_item: '', amount_cny_cents: 0 }])
  }

  async function handleSave() {
    const clean = lines
      .map((l) => ({ ...l, fee_item: l.fee_item.trim() }))
      .filter((l) => l.fee_item.length > 0)
    await adjustMut.mutateAsync({
      id: quotation.id,
      body: { lines: clean, comment: comment.trim() || undefined },
    })
    setOpen(false)
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>{trigger}</SheetTrigger>
      <SheetContent className='w-[520px] sm:max-w-none'>
        <SheetHeader>
          <SheetTitle>调价</SheetTitle>
          <SheetDescription>
            修改冻结报价的明细。总价会自动重算，保存后写入审核历史。
          </SheetDescription>
        </SheetHeader>
        <div className='flex flex-col gap-3 px-4 py-2'>
          <div className='flex flex-col gap-2'>
            {lines.map((line, idx) => (
              <div key={idx} className='flex items-center gap-2'>
                <Input
                  className='flex-1'
                  placeholder='费用项'
                  value={line.fee_item}
                  onChange={(e) => updateLine(idx, { fee_item: e.target.value })}
                />
                <Input
                  className='w-32'
                  type='number'
                  min={0}
                  value={line.amount_cny_cents}
                  onChange={(e) =>
                    updateLine(idx, { amount_cny_cents: Number(e.target.value) || 0 })
                  }
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  onClick={() => removeLine(idx)}
                  aria-label='删除行'
                >
                  <Trash className='size-4' />
                </Button>
              </div>
            ))}
          </div>
          <Button type='button' variant='outline' size='sm' onClick={addLine}>
            <Plus className='size-4' />
            新增行
          </Button>
          <div className='flex items-center justify-end text-sm text-muted-foreground'>
            合计 <span className='ml-2 font-medium text-foreground'>¥{(total / 100).toFixed(2)}</span>
          </div>
          <div className='space-y-1.5'>
            <label className='text-sm font-medium' htmlFor='adjust-comment'>
              备注（可选）
            </label>
            <Textarea
              id='adjust-comment'
              value={comment}
              onChange={(e) => setComment(e.target.value)}
            />
          </div>
        </div>
        <SheetFooter>
          <Button type='button' variant='ghost' onClick={() => setOpen(false)}>
            取消
          </Button>
          <Button
            type='button'
            onClick={handleSave}
            disabled={adjustMut.isPending || !hasValidLine}
          >
            保存
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
