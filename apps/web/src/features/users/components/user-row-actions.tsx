import { useState } from 'react'
import { MoreHorizontal } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useMe } from '@/features/auth/hooks'
import type { User } from '../types'
import { useResetUserPassword, useUpdateUser } from '../hooks'
import { UserFormDialog } from './user-form-dialog'

interface Props {
  user: User
}

export function UserRowActions({ user }: Props) {
  const me = useMe()
  const isSelf = me.data?.id === user.id
  const [editOpen, setEditOpen] = useState(false)
  const [confirmResetOpen, setConfirmResetOpen] = useState(false)
  const [generatedPassword, setGeneratedPassword] = useState<string | null>(null)

  const updateMut = useUpdateUser()
  const resetMut = useResetUserPassword()

  const nextStatus = user.status === 'active' ? 'disabled' : 'active'
  const statusActionLabel = user.status === 'active' ? '禁用' : '启用'

  const toggleStatus = () => {
    updateMut.mutate({ id: user.id, body: { status: nextStatus } })
  }

  const doReset = async () => {
    setConfirmResetOpen(false)
    try {
      const { password } = await resetMut.mutateAsync(user.id)
      setGeneratedPassword(password)
    } catch {
      /* toast in mutation */
    }
  }

  const copyPassword = async () => {
    if (!generatedPassword) return
    try {
      await navigator.clipboard.writeText(generatedPassword)
      toast.success('密码已复制到剪贴板')
    } catch {
      toast.error('复制失败，请手动选中复制')
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' size='icon' aria-label='更多操作'>
            <MoreHorizontal className='h-4 w-4' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end'>
          <DropdownMenuItem onSelect={() => setEditOpen(true)}>
            编辑
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={() => setConfirmResetOpen(true)}
            disabled={resetMut.isPending || isSelf}
          >
            重置密码{isSelf && '（禁止自改）'}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={toggleStatus}
            disabled={updateMut.isPending || isSelf}
            variant={user.status === 'active' ? 'destructive' : 'default'}
          >
            {statusActionLabel}
            {isSelf && '（禁止自改）'}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <UserFormDialog
        mode='edit'
        open={editOpen}
        onOpenChange={setEditOpen}
        initial={user}
      />

      <AlertDialog open={confirmResetOpen} onOpenChange={setConfirmResetOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认重置密码</AlertDialogTitle>
            <AlertDialogDescription>
              将为「{user.name}」生成一个新的随机密码，旧密码立即失效。请将新密码安全地传达给用户。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction onClick={doReset} disabled={resetMut.isPending}>
              {resetMut.isPending ? '重置中…' : '确认重置'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog
        open={generatedPassword !== null}
        onOpenChange={(v) => !v && setGeneratedPassword(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>新密码已生成</DialogTitle>
            <DialogDescription>
              请立即复制并安全地发送给「{user.name}」。关闭后将无法再次查看。
            </DialogDescription>
          </DialogHeader>
          <div className='flex items-center gap-2'>
            <Input
              readOnly
              value={generatedPassword ?? ''}
              className='font-mono'
              onFocus={(e) => e.target.select()}
            />
            <Button type='button' onClick={copyPassword}>
              复制
            </Button>
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setGeneratedPassword(null)}
            >
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
