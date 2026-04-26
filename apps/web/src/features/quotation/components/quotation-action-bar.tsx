import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useAuthStore } from '@/stores/auth-store'
import type { Quotation } from '../types'
import {
  useCancelQuotation,
  useCopyQuotation,
  useReviewQuotation,
  useSubmitQuotation,
  useWithdrawQuotation,
} from '../hooks'
import { QuotationAdjustSheet } from './quotation-adjust-sheet'

interface Props {
  quotation: Quotation
  onEditDraft: () => void
}

export function QuotationActionBar({ quotation, onEditDraft }: Props) {
  const user = useAuthStore((s) => s.auth.user)
  const navigate = useNavigate()
  const submit = useSubmitQuotation()
  const approve = useReviewQuotation(true)
  const reject = useReviewQuotation(false)
  const cancel = useCancelQuotation()
  const withdrawMut = useWithdrawQuotation()
  const copyMut = useCopyQuotation()

  const [commentOpen, setCommentOpen] = useState<'approve' | 'reject' | 'cancel' | null>(null)
  const [comment, setComment] = useState('')

  if (!user) return null

  const isOwner = quotation.created_by === user.id
  const isReviewer = user.role === 'reviewer' || user.role === 'admin'

  const canEdit = quotation.status === 'draft' && isOwner
  const canSubmit = quotation.status === 'draft' && isOwner
  const canCancel = quotation.status === 'draft' && isOwner
  const canReview = quotation.status === 'submitted' && isReviewer
  const canWithdraw = quotation.status === 'submitted' && isOwner
  const canAdjust = quotation.status === 'submitted' && isReviewer

  const confirmComment = async () => {
    const trimmed = comment.trim() || undefined
    if (commentOpen === 'approve') await approve.mutateAsync({ id: quotation.id, comment: trimmed })
    if (commentOpen === 'reject') await reject.mutateAsync({ id: quotation.id, comment: trimmed })
    if (commentOpen === 'cancel') await cancel.mutateAsync({ id: quotation.id, comment: trimmed })
    setCommentOpen(null)
    setComment('')
  }

  const handleCopy = async () => {
    const newDraft = await copyMut.mutateAsync(quotation.id)
    navigate({ to: '/quotations/$id', params: { id: newDraft.id } })
  }

  return (
    <>
      <div className='flex flex-wrap gap-2'>
        {canEdit && <Button variant='outline' onClick={onEditDraft}>编辑草稿</Button>}
        {canSubmit && (
          <Button onClick={() => submit.mutateAsync(quotation.id)} disabled={submit.isPending}>
            提交审核
          </Button>
        )}
        {canCancel && (
          <Button variant='ghost' onClick={() => setCommentOpen('cancel')}>
            取消草稿
          </Button>
        )}
        {canWithdraw && (
          <Button
            variant='outline'
            onClick={() => withdrawMut.mutateAsync(quotation.id)}
            disabled={withdrawMut.isPending}
          >
            撤回草稿
          </Button>
        )}
        {canAdjust && (
          <QuotationAdjustSheet
            quotation={quotation}
            trigger={<Button variant='outline'>调价</Button>}
          />
        )}
        {canReview && (
          <>
            <Button onClick={() => setCommentOpen('approve')} disabled={approve.isPending}>
              通过
            </Button>
            <Button
              variant='destructive'
              onClick={() => setCommentOpen('reject')}
              disabled={reject.isPending}
            >
              驳回
            </Button>
          </>
        )}
        <Button variant='ghost' onClick={handleCopy} disabled={copyMut.isPending}>
          复制报价
        </Button>
      </div>

      <Dialog open={commentOpen != null} onOpenChange={(o) => !o && setCommentOpen(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {commentOpen === 'approve' && '确认通过'}
              {commentOpen === 'reject' && '驳回报价'}
              {commentOpen === 'cancel' && '取消草稿'}
            </DialogTitle>
            <DialogDescription>备注（可选）将记录在状态变更日志中。</DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <Label htmlFor='comment'>备注</Label>
            <Input id='comment' value={comment} onChange={(e) => setComment(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant='ghost' onClick={() => setCommentOpen(null)}>取消</Button>
            <Button onClick={confirmComment}>确认</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
