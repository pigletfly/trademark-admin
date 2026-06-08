import { useEffect, useMemo } from 'react'
import { z } from 'zod'
import {
  useForm,
  type Control,
  type FieldValues,
  type Path,
} from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Save } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
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
import { Textarea } from '@/components/ui/textarea'
import type { Country } from '@/features/catalog/types'
import {
  useCreateOrReplaceMadridPricing,
  useCreateOrReplaceSingleClassPricing,
} from '../hooks'
import type { MadridPricingEntry, SingleClassPricingEntry } from '../types'

const moneySchema = z
  .string()
  .refine((value) => /^\d+(\.\d{1,2})?$/.test(value) && Number(value) >= 0, {
    message: '请输入非负金额，最多两位小数',
  })

const integerMoneySchema = z
  .string()
  .refine((value) => /^\d+(\.\d{1,2})?$/.test(value) && Number(value) >= 0, {
    message: '请输入非负金额，最多两位小数',
  })

const dateSchema = z
  .string()
  .refine((value) => /^\d{4}-\d{2}-\d{2}$/.test(value), {
    message: '格式必须是 YYYY-MM-DD',
  })

const optionalIntegerSchema = z.string().refine(
  (value) => {
    const trimmed = value.trim()
    return trimmed === '' || /^\d+$/.test(trimmed)
  },
  { message: '请输入非负整数' }
)

const madridSchema = z.object({
  country_id: z.string().optional(),
  sequence_no: optionalIntegerSchema.optional(),
  country_area: z.string().min(1, '请输入国家/地区或基础费名称'),
  official_fee_chf: integerMoneySchema,
  agency_fee_cny: moneySchema,
  notes: z.string().optional().or(z.literal('')),
  effective_from: dateSchema,
})

const singleClassSchema = z.object({
  country_id: z.string().min(1, '请选择国家/地区'),
  continent: z.string().min(1, '请输入大洲'),
  country_area: z.string().min(1, '请输入国家/地区'),
  first_class_fee_cny: moneySchema,
  first_class_fee_tax6_cny: moneySchema,
  first_class_fee_tax1_cny: moneySchema,
  additional_class_fee_cny: moneySchema,
  additional_class_fee_tax6_cny: moneySchema,
  additional_class_fee_tax1_cny: moneySchema,
  required_documents: z.string().optional().or(z.literal('')),
  notarization_fee: z.string().optional().or(z.literal('')),
  acceptance_time: z.string().optional().or(z.literal('')),
  registration_months: z.string().optional().or(z.literal('')),
  validity_years: optionalIntegerSchema.optional(),
  note1: z.string().optional().or(z.literal('')),
  note2: z.string().optional().or(z.literal('')),
  effective_from: dateSchema,
})

type MadridValues = z.infer<typeof madridSchema>
type SingleClassValues = z.infer<typeof singleClassSchema>

const MADRID_BASE_COUNTRY_AREA = 'Basic registration fee - black and white mark'

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function centsToYuan(cents: number | null | undefined): string {
  return ((cents ?? 0) / 100).toFixed(2)
}

function yuanToCents(value: string): number {
  return Math.round(Number(value) * 100)
}

export function MadridPricingDrawer({
  open,
  onOpenChange,
  countryId,
  countries,
  onSavedCountryID,
  mode,
  current,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  countries: Country[]
  onSavedCountryID?: (countryId: string) => void
  mode: 'base' | 'country'
  current?: MadridPricingEntry
}) {
  const form = useForm<MadridValues>({
    resolver: zodResolver(madridSchema),
    defaultValues: {
      country_id: '',
      sequence_no: '',
      country_area: '',
      official_fee_chf: '0.00',
      agency_fee_cny: '0.00',
      notes: '',
      effective_from: todayISO(),
    },
  })
  const mutation = useCreateOrReplaceMadridPricing()
  const madridCountries = useMemo(
    () => countries.filter((country) => country.is_madrid_member),
    [countries]
  )

  useEffect(() => {
    if (!open) return
    const defaultCountryID = current?.country_id ?? countryId
    const defaultCountry = madridCountries.find(
      (country) => country.id === defaultCountryID
    )
    form.reset({
      country_id: mode === 'country' ? defaultCountryID : '',
      sequence_no:
        current?.sequence_no == null ? '' : String(current.sequence_no),
      country_area:
        current?.country_area ??
        (mode === 'base' ? MADRID_BASE_COUNTRY_AREA : defaultCountry?.name_zh ?? ''),
      official_fee_chf: centsToYuan(current?.official_fee_chf_cents),
      agency_fee_cny: centsToYuan(current?.agency_fee_cny_cents),
      notes: current?.notes ?? '',
      effective_from: todayISO(),
    })
  }, [countryId, current, form, madridCountries, mode, open])

  const onSubmit = form.handleSubmit(async (values) => {
    const selectedCountry = madridCountries.find(
      (country) => country.id === values.country_id
    )
    const saved = await mutation
      .mutateAsync({
        country_id: mode === 'country' ? values.country_id || countryId : null,
        sequence_no:
          mode === 'country' && values.sequence_no?.trim()
            ? Number(values.sequence_no)
            : null,
        country_area:
          mode === 'base'
            ? values.country_area.trim() || MADRID_BASE_COUNTRY_AREA
            : selectedCountry?.name_zh ?? values.country_area.trim(),
        official_fee_chf_cents: yuanToCents(values.official_fee_chf),
        agency_fee_cny_cents: yuanToCents(values.agency_fee_cny),
        is_base_fee: mode === 'base',
        notes: values.notes?.trim() ? values.notes.trim() : null,
        effective_from: values.effective_from,
      })
      .then((entry) => {
        if (mode === 'country' && entry.country_id) {
          onSavedCountryID?.(entry.country_id)
        }
        return entry
      })
      .catch(() => {})
    if (saved) {
      onOpenChange(false)
    }
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-xl'>
        <SheetHeader>
          <SheetTitle>
            {mode === 'base' ? '马德里基础费' : '马德里国家费'}
          </SheetTitle>
          <SheetDescription>
            保存会生成新版本并废止当前生效版本。
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            {mode === 'country' && (
              <TextField
                control={form.control}
                name='sequence_no'
                label='序号'
              />
            )}
            {mode === 'country' && (
              <FormField
                control={form.control}
                name='country_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>国家/地区</FormLabel>
                    <Select
                      value={field.value ?? ''}
                      onValueChange={(value) => {
                        field.onChange(value)
                        const country = madridCountries.find(
                          (item) => item.id === value
                        )
                        form.setValue('country_area', country?.name_zh ?? '', {
                          shouldValidate: true,
                        })
                      }}
                      disabled={madridCountries.length === 0}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='选择支持马德里申请的国家' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {madridCountries.map((country) => (
                          <SelectItem key={country.id} value={country.id}>
                            {country.name_zh} · {country.code}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <TextField
              control={form.control}
              name='official_fee_chf'
              label='官费（瑞士法郎）'
            />
            <TextField
              control={form.control}
              name='agency_fee_cny'
              label='我所代理费（人民币元）'
            />
            <TextAreaField
              control={form.control}
              name='notes'
              label='备注'
              rows={3}
            />
            <TextField
              control={form.control}
              name='effective_from'
              label='生效日期'
              type='date'
            />
            <SheetFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={mutation.isPending}
              >
                取消
              </Button>
              <Button type='submit' disabled={mutation.isPending}>
                <Save className='mr-2 h-4 w-4' />
                {mutation.isPending ? '保存中…' : '保存'}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

export function SingleClassPricingDrawer({
  open,
  onOpenChange,
  countryId,
  countries,
  onSavedCountryID,
  current,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  countries: Country[]
  onSavedCountryID?: (countryId: string) => void
  current?: SingleClassPricingEntry
}) {
  const form = useForm<SingleClassValues>({
    resolver: zodResolver(singleClassSchema),
    defaultValues: {
      country_id: '',
      continent: '',
      country_area: '',
      first_class_fee_cny: '0.00',
      first_class_fee_tax6_cny: '0.00',
      first_class_fee_tax1_cny: '0.00',
      additional_class_fee_cny: '0.00',
      additional_class_fee_tax6_cny: '0.00',
      additional_class_fee_tax1_cny: '0.00',
      required_documents: '',
      notarization_fee: '',
      acceptance_time: '',
      registration_months: '',
      validity_years: '',
      note1: '',
      note2: '',
      effective_from: todayISO(),
    },
  })
  const mutation = useCreateOrReplaceSingleClassPricing()

  useEffect(() => {
    if (!open) return
    const defaultCountryID = current?.country_id ?? countryId
    const defaultCountry = countries.find(
      (country) => country.id === defaultCountryID
    )
    form.reset({
      country_id: defaultCountryID,
      continent: current?.continent ?? '',
      country_area: current?.country_area ?? defaultCountry?.name_zh ?? '',
      first_class_fee_cny: centsToYuan(current?.first_class_fee_cny_cents),
      first_class_fee_tax6_cny: centsToYuan(
        current?.first_class_fee_tax6_cny_cents
      ),
      first_class_fee_tax1_cny: centsToYuan(
        current?.first_class_fee_tax1_cny_cents
      ),
      additional_class_fee_cny: centsToYuan(
        current?.additional_class_fee_cny_cents
      ),
      additional_class_fee_tax6_cny: centsToYuan(
        current?.additional_class_fee_tax6_cny_cents
      ),
      additional_class_fee_tax1_cny: centsToYuan(
        current?.additional_class_fee_tax1_cny_cents
      ),
      required_documents: current?.required_documents ?? '',
      notarization_fee: current?.notarization_fee ?? '',
      acceptance_time: current?.acceptance_time ?? '',
      registration_months: current?.registration_months ?? '',
      validity_years:
        current?.validity_years == null ? '' : String(current.validity_years),
      note1: current?.note1 ?? '',
      note2: current?.note2 ?? '',
      effective_from: todayISO(),
    })
  }, [countries, countryId, current, form, open])

  const onSubmit = form.handleSubmit(async (values) => {
    const selectedCountry = countries.find(
      (country) => country.id === values.country_id
    )
    const saved = await mutation
      .mutateAsync({
        country_id: values.country_id || countryId,
        continent: values.continent.trim(),
        country_area: selectedCountry?.name_zh ?? values.country_area.trim(),
        first_class_fee_cny_cents: yuanToCents(values.first_class_fee_cny),
        first_class_fee_tax6_cny_cents: yuanToCents(
          values.first_class_fee_tax6_cny
        ),
        first_class_fee_tax1_cny_cents: yuanToCents(
          values.first_class_fee_tax1_cny
        ),
        additional_class_fee_cny_cents: yuanToCents(
          values.additional_class_fee_cny
        ),
        additional_class_fee_tax6_cny_cents: yuanToCents(
          values.additional_class_fee_tax6_cny
        ),
        additional_class_fee_tax1_cny_cents: yuanToCents(
          values.additional_class_fee_tax1_cny
        ),
        required_documents: values.required_documents ?? '',
        notarization_fee: values.notarization_fee ?? '',
        acceptance_time: values.acceptance_time ?? '',
        registration_months: values.registration_months ?? '',
        validity_years: values.validity_years?.trim()
          ? Number(values.validity_years)
          : null,
        note1: values.note1?.trim() ? values.note1.trim() : null,
        note2: values.note2?.trim() ? values.note2.trim() : null,
        effective_from: values.effective_from,
      })
      .then((entry) => {
        onSavedCountryID?.(entry.country_id)
        return entry
      })
      .catch(() => {})
    if (saved) {
      onOpenChange(false)
    }
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full overflow-y-auto sm:max-w-2xl'>
        <SheetHeader>
          <SheetTitle>单一分类定价</SheetTitle>
          <SheetDescription>
            字段对应报价表的“全单一途径注册申请”sheet。
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='flex flex-col gap-4 p-4'>
            <div className='grid gap-4 sm:grid-cols-2'>
              <TextField control={form.control} name='continent' label='大洲' />
              <FormField
                control={form.control}
                name='country_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>国家/地区</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={(value) => {
                        field.onChange(value)
                        const country = countries.find(
                          (item) => item.id === value
                        )
                        form.setValue('country_area', country?.name_zh ?? '', {
                          shouldValidate: true,
                        })
                      }}
                      disabled={countries.length === 0}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='选择国家/地区' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {countries.map((country) => (
                          <SelectItem key={country.id} value={country.id}>
                            {country.name_zh} · {country.code}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <TextField
                control={form.control}
                name='first_class_fee_cny'
                label='首类费用（不含税）'
              />
              <TextField
                control={form.control}
                name='first_class_fee_tax6_cny'
                label='首类费用（含税 6%）'
              />
              <TextField
                control={form.control}
                name='first_class_fee_tax1_cny'
                label='首类费用（含税 1%）'
              />
              <TextField
                control={form.control}
                name='additional_class_fee_cny'
                label='每次类费用（不含税）'
              />
              <TextField
                control={form.control}
                name='additional_class_fee_tax6_cny'
                label='每次类费用（含税 6%）'
              />
              <TextField
                control={form.control}
                name='additional_class_fee_tax1_cny'
                label='每次类费用（含税 1%）'
              />
              <TextField
                control={form.control}
                name='acceptance_time'
                label='受理需时'
              />
              <TextField
                control={form.control}
                name='registration_months'
                label='注册需时（月）'
              />
              <TextField
                control={form.control}
                name='validity_years'
                label='有效期（年）'
              />
              <TextField
                control={form.control}
                name='effective_from'
                label='生效日期'
                type='date'
              />
            </div>
            <TextAreaField
              control={form.control}
              name='required_documents'
              label='所需文件'
              rows={3}
            />
            <TextAreaField
              control={form.control}
              name='notarization_fee'
              label='公认证费'
              rows={2}
            />
            <TextAreaField
              control={form.control}
              name='note1'
              label='备注 1'
              rows={2}
            />
            <TextAreaField
              control={form.control}
              name='note2'
              label='备注 2'
              rows={2}
            />
            <SheetFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={mutation.isPending}
              >
                取消
              </Button>
              <Button type='submit' disabled={mutation.isPending}>
                <Save className='mr-2 h-4 w-4' />
                {mutation.isPending ? '保存中…' : '保存'}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}

function TextField<T extends FieldValues>({
  control,
  name,
  label,
  type = 'text',
}: {
  control: Control<T>
  name: Path<T>
  label: string
  type?: string
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input type={type} {...field} />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function TextAreaField<T extends FieldValues>({
  control,
  name,
  label,
  rows,
}: {
  control: Control<T>
  name: Path<T>
  label: string
  rows: number
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Textarea rows={rows} {...field} />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
