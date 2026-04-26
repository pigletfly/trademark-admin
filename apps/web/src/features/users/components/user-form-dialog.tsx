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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { useMe } from '@/features/auth/hooks'
import type {
  CreateUserRequest,
  UpdateUserRequest,
  User,
  UserRole,
  UserStatus,
} from '../types'
import { useCreateUser, useUpdateUser } from '../hooks'

const ROLE_VALUES = ['salesperson', 'reviewer', 'admin'] as const
const STATUS_VALUES = ['active', 'disabled'] as const

const createSchema = z.object({
  name: z.string().min(1, '姓名不能为空').max(100, '姓名过长'),
  email: z.string().email('邮箱格式不正确').max(200),
  phone: z.string().max(50).optional().or(z.literal('')),
  role: z.enum(ROLE_VALUES),
  password: z.string().min(8, '密码至少 8 位').max(200),
})

const editSchema = z.object({
  name: z.string().min(1, '姓名不能为空').max(100, '姓名过长'),
  phone: z.string().max(50).optional().or(z.literal('')),
  role: z.enum(ROLE_VALUES),
  status: z.enum(STATUS_VALUES),
})

type CreateFormValues = z.infer<typeof createSchema>
type EditFormValues = z.infer<typeof editSchema>

const ROLE_OPTIONS: { value: UserRole; label: string }[] = [
  { value: 'salesperson', label: '业务员' },
  { value: 'reviewer', label: '国际部商务' },
  { value: 'admin', label: '管理员' },
]

const STATUS_OPTIONS: { value: UserStatus; label: string }[] = [
  { value: 'active', label: '启用' },
  { value: 'disabled', label: '禁用' },
]

interface Props {
  mode: 'create' | 'edit'
  open: boolean
  onOpenChange: (open: boolean) => void
  initial?: User
}

export function UserFormDialog({ mode, open, onOpenChange, initial }: Props) {
  if (mode === 'edit' && initial) {
    return (
      <EditDialog
        open={open}
        onOpenChange={onOpenChange}
        initial={initial}
      />
    )
  }
  return <CreateDialog open={open} onOpenChange={onOpenChange} />
}

function CreateDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const form = useForm<CreateFormValues>({
    resolver: zodResolver(createSchema),
    defaultValues: {
      name: '',
      email: '',
      phone: '',
      role: 'salesperson',
      password: '',
    },
  })
  const createMut = useCreateUser()

  useEffect(() => {
    if (!open) form.reset()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const onSubmit = form.handleSubmit(async (values) => {
    const payload: CreateUserRequest = {
      name: values.name,
      email: values.email,
      phone: values.phone || undefined,
      role: values.role,
      password: values.password,
    }
    try {
      await createMut.mutateAsync(payload)
      onOpenChange(false)
    } catch {
      /* toast in mutation */
    }
  })

  const busy = createMut.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>新建用户</DialogTitle>
          <DialogDescription>填写信息并保存；初始密码将由管理员告知用户。</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='grid grid-cols-2 gap-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>姓名</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='email'
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
              name='phone'
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
              name='role'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>角色</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {ROLE_OPTIONS.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                          {o.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='password'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>初始密码</FormLabel>
                  <FormControl>
                    <Input type='text' placeholder='至少 8 位' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter className='col-span-2'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={busy}
              >
                取消
              </Button>
              <Button type='submit' disabled={busy}>
                {busy ? '创建中…' : '创建'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}

function EditDialog({
  open,
  onOpenChange,
  initial,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial: User
}) {
  const me = useMe()
  const isSelf = me.data?.id === initial.id
  const form = useForm<EditFormValues>({
    resolver: zodResolver(editSchema),
    defaultValues: {
      name: initial.name,
      phone: initial.phone ?? '',
      role: initial.role,
      status: initial.status,
    },
  })
  const updateMut = useUpdateUser()

  useEffect(() => {
    if (open) {
      form.reset({
        name: initial.name,
        phone: initial.phone ?? '',
        role: initial.role,
        status: initial.status,
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initial.id])

  const onSubmit = form.handleSubmit(async (values) => {
    const payload: UpdateUserRequest = {
      name: values.name,
      phone: values.phone || undefined,
    }
    // Actor cannot change their own role/status — strip them from the payload
    // regardless of what the (disabled) selects hold.
    if (!isSelf) {
      payload.role = values.role
      payload.status = values.status
    }
    try {
      await updateMut.mutateAsync({ id: initial.id, body: payload })
      onOpenChange(false)
    } catch {
      /* toast in mutation */
    }
  })

  const busy = updateMut.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>编辑用户</DialogTitle>
          <DialogDescription>
            {isSelf
              ? '邮箱、角色、状态、密码均不可自改，请请其他管理员处理。'
              : '邮箱与密码不可在此修改。'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='grid grid-cols-2 gap-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>姓名</FormLabel>
                  <FormControl>
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormItem className='col-span-2'>
              <FormLabel>邮箱</FormLabel>
              <Input value={initial.email} disabled readOnly />
            </FormItem>
            <FormField
              control={form.control}
              name='phone'
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
              name='role'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>角色</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={isSelf}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {ROLE_OPTIONS.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                          {o.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem className='col-span-2'>
                  <FormLabel>状态</FormLabel>
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                    disabled={isSelf}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {STATUS_OPTIONS.map((o) => (
                        <SelectItem key={o.value} value={o.value}>
                          {o.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter className='col-span-2'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={busy}
              >
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
