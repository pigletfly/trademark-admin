import { useEffect } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import type { Customer } from '../types'
import { useCreateCustomer, useUpdateCustomer } from '../hooks'

const schema = z.object({
  name: z.string().min(1, '客户名称不能为空').max(200, '客户名称过长'),
  industry: z.string().max(200).optional().or(z.literal('')),
  is_returning: z.boolean(),
  price_sensitive: z.boolean(),
  contact_name: z.string().max(100).optional().or(z.literal('')),
  contact_phone: z.string().max(50).optional().or(z.literal('')),
  contact_email: z.string().email('邮箱格式不正确').max(200).optional().or(z.literal('')),
  notes: z.string().max(2000).optional().or(z.literal('')),
})

type FormValues = z.infer<typeof schema>

interface Props {
  mode: 'create' | 'edit'
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: Customer
}

const emptyValues: FormValues = {
  name: '',
  industry: '',
  is_returning: false,
  price_sensitive: false,
  contact_name: '',
  contact_phone: '',
  contact_email: '',
  notes: '',
}

export function CustomerFormDialog({ mode, open, onOpenChange, initial }: Props) {
  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: emptyValues,
  })
  const createMut = useCreateCustomer()
  const updateMut = useUpdateCustomer()

  useEffect(() => {
    if (!open) return
    if (mode === 'edit' && initial) {
      form.reset({
        name: initial.name,
        industry: initial.industry ?? '',
        is_returning: initial.is_returning,
        price_sensitive: initial.price_sensitive,
        contact_name: initial.contact_name ?? '',
        contact_phone: initial.contact_phone ?? '',
        contact_email: initial.contact_email ?? '',
        notes: initial.notes ?? '',
      })
    } else {
      form.reset(emptyValues)
    }
  }, [open, mode, initial, form])

  const onSubmit = form.handleSubmit(async (values) => {
    // Normalise empty strings to null so backend stores NULL, not "".
    const payload = {
      name: values.name,
      industry: values.industry || null,
      is_returning: values.is_returning,
      price_sensitive: values.price_sensitive,
      contact_name: values.contact_name || null,
      contact_phone: values.contact_phone || null,
      contact_email: values.contact_email || null,
      notes: values.notes || null,
    }

    try {
      if (mode === 'edit' && initial) {
        await updateMut.mutateAsync({ id: initial.id, body: payload })
      } else {
        await createMut.mutateAsync(payload)
      }
      onOpenChange(false)
    } catch {
      // Toast is already shown by the mutation's onError; keep dialog open.
    }
  })

  const busy = createMut.isPending || updateMut.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{mode === 'edit' ? '编辑客户' : '新建客户'}</DialogTitle>
          <DialogDescription>
            {mode === 'edit' ? '修改客户信息后点击保存。' : '填写客户基本信息并保存。'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='grid grid-cols-2 gap-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>客户名称</FormLabel>
                  <FormControl>
                    <Input placeholder='必填' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='industry'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>行业</FormLabel>
                  <FormControl>
                    <Input placeholder='例如 软件、零售、制造' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='is_returning'
              render={({ field }) => (
                <FormItem className='flex items-center gap-2 col-span-1'>
                  <FormControl>
                    <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                  </FormControl>
                  <FormLabel className='!m-0'>回头客户</FormLabel>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='price_sensitive'
              render={({ field }) => (
                <FormItem className='flex items-center gap-2 col-span-1'>
                  <FormControl>
                    <Checkbox checked={field.value} onCheckedChange={(v) => field.onChange(!!v)} />
                  </FormControl>
                  <FormLabel className='!m-0'>价格敏感</FormLabel>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>联系人</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_phone'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>电话</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='contact_email'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>邮箱</FormLabel>
                  <FormControl>
                    <Input type='email' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='notes'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>备注</FormLabel>
                  <FormControl>
                    <Textarea rows={3} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter className='col-span-2'>
              <Button type='button' variant='outline' onClick={() => onOpenChange(false)} disabled={busy}>
                取消
              </Button>
              <Button type='submit' disabled={busy}>
                {busy ? '保存中…' : '保存'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
