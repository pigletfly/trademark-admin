import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useCustomersList } from '@/features/customers/hooks'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import type { Quotation, ServiceTier } from '../types'
import { useCreateQuotation, useUpdateQuotationDraft } from '../hooks'

// Customer + country come from existing catalog features. Quotation
// draft form only needs to pick IDs + tier + optional notes.
const schema = z.object({
  customer_id: z.string().uuid({ message: '请选择客户' }),
  country_id: z.string().uuid({ message: '请选择国家' }),
  service_tier: z.enum(['basic', 'standard', 'premium']),
  notes: z.string().optional().or(z.literal('')),
})
type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (v: boolean) => void
  initial?: Quotation
}

export function QuotationFormSheet({ open, onOpenChange, initial }: Props) {
  const isEdit = Boolean(initial)
  const { data: customers } = useCustomersList({ page: 1, page_size: 100 })
  const { data: countries } = useCountries()
  const create = useCreateQuotation()
  const update = useUpdateQuotationDraft()

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      customer_id: initial?.customer_id ?? '',
      country_id: initial?.country_id ?? '',
      service_tier: initial?.service_tier ?? 'basic',
      notes: initial?.notes ?? '',
    },
  })

  useEffect(() => {
    if (open) {
      form.reset({
        customer_id: initial?.customer_id ?? '',
        country_id: initial?.country_id ?? '',
        service_tier: initial?.service_tier ?? 'basic',
        notes: initial?.notes ?? '',
      })
    }
  }, [open, initial, form])

  const onSubmit = form.handleSubmit(async (values) => {
    const payload = { ...values, notes: values.notes || null }
    if (isEdit && initial) {
      await update.mutateAsync({ id: initial.id, body: payload })
    } else {
      await create.mutateAsync(payload)
    }
    onOpenChange(false)
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle>{isEdit ? '编辑报价草稿' : '新建报价'}</SheetTitle>
          <SheetDescription>
            选择客户、国家和服务级别。提交后将按当前定价冻结金额。
          </SheetDescription>
        </SheetHeader>
        <form onSubmit={onSubmit} className='flex flex-col gap-4 px-4 py-2'>
          <div className='space-y-1.5'>
            <Label htmlFor='customer_id'>客户</Label>
            <Select
              value={form.watch('customer_id')}
              onValueChange={(v) => form.setValue('customer_id', v, { shouldValidate: true })}
            >
              <SelectTrigger id='customer_id'>
                <SelectValue placeholder='请选择客户' />
              </SelectTrigger>
              <SelectContent>
                {customers?.items.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {form.formState.errors.customer_id && (
              <p className='text-xs text-destructive'>{form.formState.errors.customer_id.message}</p>
            )}
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='country_id'>国家</Label>
            <Select
              value={form.watch('country_id')}
              onValueChange={(v) => form.setValue('country_id', v, { shouldValidate: true })}
            >
              <SelectTrigger id='country_id'>
                <SelectValue placeholder='请选择国家' />
              </SelectTrigger>
              <SelectContent>
                {countries?.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name_zh}（{c.code}）
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {form.formState.errors.country_id && (
              <p className='text-xs text-destructive'>{form.formState.errors.country_id.message}</p>
            )}
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='service_tier'>服务级别</Label>
            <Select
              value={form.watch('service_tier')}
              onValueChange={(v) => form.setValue('service_tier', v as ServiceTier, { shouldValidate: true })}
            >
              <SelectTrigger id='service_tier'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='basic'>basic</SelectItem>
                <SelectItem value='standard'>standard</SelectItem>
                <SelectItem value='premium'>premium</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='notes'>备注</Label>
            <Input id='notes' {...form.register('notes')} />
          </div>
          <SheetFooter>
            <Button type='button' variant='ghost' onClick={() => onOpenChange(false)}>
              取消
            </Button>
            <Button type='submit' disabled={create.isPending || update.isPending}>
              保存
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}
