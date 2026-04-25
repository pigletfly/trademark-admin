import { useEffect } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import type { PricingEntry, ServiceTier } from '../types'
import { SERVICE_TIER_LABEL_ZH } from '../types'
import { useCreateOrReplacePricing, useDeprecatePricing } from '../hooks'

// amount in 元 (e.g. 120.50), converted to cents on submit
const schema = z.object({
  amount_cny_yuan: z.string().refine(
    (v) => /^\d+(\.\d{1,2})?$/.test(v) && Number(v) >= 0,
    { message: '请输入非负金额，最多两位小数' }
  ),
  effective_from: z.string().refine(
    (v) => /^\d{4}-\d{2}-\d{2}$/.test(v),
    { message: '格式必须是 YYYY-MM-DD' }
  ),
  notes: z.string().max(2000).optional().or(z.literal('')),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  feeItem: string
  serviceTier: ServiceTier
  current?: PricingEntry
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

export function PricingEntryDrawer({
  open,
  onOpenChange,
  countryId,
  feeItem,
  serviceTier,
  current,
}: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { amount_cny_yuan: '0', effective_from: todayISO(), notes: '' },
  })
  const createMut = useCreateOrReplacePricing()
  const deprecateMut = useDeprecatePricing()

  useEffect(() => {
    if (!open) return
    if (current) {
      form.reset({
        amount_cny_yuan: (current.amount_cny_cents / 100).toFixed(2),
        effective_from: todayISO(),
        notes: current.notes ?? '',
      })
    } else {
      form.reset({ amount_cny_yuan: '0', effective_from: todayISO(), notes: '' })
    }
  }, [open, current, form])

  const onSubmit = form.handleSubmit(async (values) => {
    const cents = Math.round(Number(values.amount_cny_yuan) * 100)
    await createMut
      .mutateAsync({
        country_id: countryId,
        service_tier: serviceTier,
        fee_item: feeItem,
        amount_cny_cents: cents,
        notes: values.notes || null,
        effective_from: values.effective_from,
      })
      .then(() => onOpenChange(false))
      .catch(() => { /* toast already shown */ })
  })

  const onDeprecate = async () => {
    if (!current) return
    await deprecateMut
      .mutateAsync({ id: current.id })
      .then(() => onOpenChange(false))
      .catch(() => { /* toast already shown */ })
  }

  const busy = createMut.isPending || deprecateMut.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-lg overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>
            {current ? '修改' : '新增'}定价 · {feeItem} · {SERVICE_TIER_LABEL_ZH[serviceTier]}
          </SheetTitle>
          <SheetDescription>
            {current
              ? '保存会生成一条新版本并自动废止当前生效版本。'
              : '填写金额与生效日期。'}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            <FormField
              control={form.control}
              name='amount_cny_yuan'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>金额（人民币元）</FormLabel>
                  <FormControl>
                    <Input inputMode='decimal' placeholder='0.00' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='effective_from'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>生效日期</FormLabel>
                  <FormControl>
                    <Input type='date' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SheetFooter className='flex flex-row items-center justify-between gap-2'>
              {current && (
                <Button
                  type='button'
                  variant='destructive'
                  onClick={onDeprecate}
                  disabled={busy}
                >
                  废止当前版本
                </Button>
              )}
              <div className='ms-auto flex gap-2'>
                <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={busy}>
                  取消
                </Button>
                <Button type='submit' disabled={busy}>
                  {busy ? '保存中…' : '保存'}
                </Button>
              </div>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
