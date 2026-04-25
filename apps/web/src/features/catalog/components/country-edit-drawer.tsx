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
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import type { Country } from '../types'
import { useUpdateCountry } from '../hooks'

const numericStringSchema = (max: number) =>
  z
    .string()
    .refine(
      (v) => {
        if (v === '') return true
        const n = Number(v)
        return Number.isInteger(n) && n >= 0 && n <= max
      },
      { message: `请输入 0 至 ${max} 之间的整数` }
    )

const schema = z.object({
  name_zh: z.string().min(1, '中文名不能为空').max(100),
  name_en: z.string().min(1, '英文名不能为空').max(200),
  is_madrid_member: z.boolean(),
  default_acceptance_days: numericStringSchema(3650),
  default_registration_months: numericStringSchema(240),
  requires_notarization: z.boolean(),
  notes_zh: z.string().max(1000).or(z.literal('')),
  notes_en: z.string().max(1000).or(z.literal('')),
  sort_order: z.string().refine(
    (v) => {
      const n = Number(v)
      return v !== '' && Number.isInteger(n) && n >= 0 && n <= 10_000
    },
    { message: '请输入 0 至 10000 之间的整数' }
  ),
  enabled: z.boolean(),
})

type FormValues = z.infer<typeof schema>

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  country: Country | null
}

export function CountryEditDrawer({ open, onOpenChange, country }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name_zh: '',
      name_en: '',
      is_madrid_member: false,
      default_acceptance_days: '',
      default_registration_months: '',
      requires_notarization: false,
      notes_zh: '',
      notes_en: '',
      sort_order: '0',
      enabled: true,
    },
  })
  const updateMut = useUpdateCountry()

  useEffect(() => {
    if (!open || !country) return
    form.reset({
      name_zh: country.name_zh,
      name_en: country.name_en,
      is_madrid_member: country.is_madrid_member,
      default_acceptance_days:
        country.default_acceptance_days == null ? '' : String(country.default_acceptance_days),
      default_registration_months:
        country.default_registration_months == null ? '' : String(country.default_registration_months),
      requires_notarization: country.requires_notarization,
      notes_zh: country.notes_zh ?? '',
      notes_en: country.notes_en ?? '',
      sort_order: String(country.sort_order),
      enabled: country.enabled,
    })
  }, [open, country, form])

  const onSubmit = form.handleSubmit(async (values) => {
    if (!country) return
    await updateMut
      .mutateAsync({
        id: country.id,
        body: {
          name_zh: values.name_zh,
          name_en: values.name_en,
          is_madrid_member: values.is_madrid_member,
          default_acceptance_days:
            values.default_acceptance_days === '' ? null : Number(values.default_acceptance_days),
          default_registration_months:
            values.default_registration_months === '' ? null : Number(values.default_registration_months),
          requires_notarization: values.requires_notarization,
          notes_zh: values.notes_zh || null,
          notes_en: values.notes_en || null,
          sort_order: Number(values.sort_order),
          enabled: values.enabled,
        },
      })
      .then(() => onOpenChange(false))
      .catch(() => {
        /* toast shown inside hook */
      })
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-xl overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>{country ? `编辑国家 · ${country.code}` : '编辑国家'}</SheetTitle>
          <SheetDescription>修改后点击保存。ISO 代码不可更改。</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            <div className='grid grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='name_zh'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>中文名</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='name_en'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>英文名</FormLabel>
                    <FormControl>
                      <Input {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='default_acceptance_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>默认受理天数</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='default_registration_months'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>默认注册月数</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} value={field.value ?? ''} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='sort_order'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>排序权重</FormLabel>
                    <FormControl>
                      <Input type='number' min={0} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='is_madrid_member'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>Madrid 成员国</FormLabel>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='requires_notarization'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>需要公证</FormLabel>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                    </FormControl>
                    <FormLabel className='!m-0'>启用</FormLabel>
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='notes_zh'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>中文备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes_en'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>英文备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <SheetFooter>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={updateMut.isPending}>
                取消
              </Button>
              <Button type='submit' disabled={updateMut.isPending}>
                {updateMut.isPending ? '保存中…' : '保存'}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
