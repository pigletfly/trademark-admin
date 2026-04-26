import { useNavigate } from '@tanstack/react-router'
import { AxiosError } from 'axios'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { AlertCircle, Loader2 } from 'lucide-react'
import {
  useCreateQuotation,
  useCreateAndSubmit,
  useUpdateQuotationDraft,
  useUpdateAndSubmit,
} from '../../hooks/use-quotation-mutations'
import { usePreview } from '../hooks/use-preview'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
  onExit: () => void // called after a successful submit to clear the store
}

// Step 5: preview and submit. Two paths:
//   - 保存草稿 → POST /quotations (or PATCH in edit mode)
//   - 保存并提交 → POST + POST /submit (or PATCH + POST /submit)
// After any successful action, onExit() resets the store and the caller
// navigates to the detail page.
export function StepPreview({ state, onExit }: Props) {
  const navigate = useNavigate()
  const preview = usePreview({
    customer_id: state.draft.customer_id,
    country_id: state.draft.country_id,
    service_tier: state.draft.service_tier,
  })
  const createMut = useCreateQuotation()
  const createSubmitMut = useCreateAndSubmit()
  const updateMut = useUpdateQuotationDraft()
  const updateSubmitMut = useUpdateAndSubmit()

  const isEdit = state.editingId !== null
  const busy =
    createMut.isPending ||
    createSubmitMut.isPending ||
    updateMut.isPending ||
    updateSubmitMut.isPending
  const canSubmit = preview.isSuccess && !busy

  const body = {
    customer_id: state.draft.customer_id,
    country_id: state.draft.country_id,
    service_tier: state.draft.service_tier,
    notes: state.draft.notes ? state.draft.notes : null,
  }

  const saveDraft = async () => {
    if (isEdit && state.editingId) {
      await updateMut.mutateAsync({ id: state.editingId, body })
      onExit()
      navigate({ to: '/quotations/$id', params: { id: state.editingId } })
    } else {
      const q = await createMut.mutateAsync(body)
      onExit()
      navigate({ to: '/quotations/$id', params: { id: q.id } })
    }
  }

  const saveAndSubmit = async () => {
    if (isEdit && state.editingId) {
      const result = await updateSubmitMut.mutateAsync({ id: state.editingId, body })
      onExit()
      navigate({ to: '/quotations/$id', params: { id: result.id } })
    } else {
      const result = await createSubmitMut.mutateAsync(body)
      onExit()
      navigate({ to: '/quotations/$id', params: { id: result.id } })
    }
  }

  // Error path: show a retry button. The two save buttons stay
  // disabled because saving without a valid signature+total is the
  // same thing as submitting a broken quotation.
  if (preview.isError) {
    const code = (preview.error as AxiosError<{ code?: string }>)?.response?.data?.code
    const message =
      code === 'ERR_MISSING_PRICING'
        ? '该国家/级别暂无定价,请联系管理员或回到上一步选择其他国家'
        : code === 'ERR_NOT_FOUND'
          ? '客户不存在,请回到第 1 步重新选择'
          : '预览失败,请稍后重试'
    return (
      <div className='flex flex-col gap-3'>
        <Alert variant='destructive'>
          <AlertCircle className='h-4 w-4' />
          <AlertTitle>预览失败 / Preview failed</AlertTitle>
          <AlertDescription>{message}</AlertDescription>
        </Alert>
        <Button variant='outline' onClick={() => preview.refetch()}>
          重试 / Retry
        </Button>
        <div className='flex justify-end gap-2'>
          <Button disabled variant='outline'>
            {isEdit ? '保存修改' : '保存草稿'}
          </Button>
          <Button disabled>
            保存并提交
          </Button>
        </div>
      </div>
    )
  }

  if (preview.isLoading || !preview.data) {
    return (
      <div className='flex items-center gap-2 text-sm text-muted-foreground'>
        <Loader2 className='h-4 w-4 animate-spin' /> 计算中 / Computing…
      </div>
    )
  }

  const { lines, total_cny_cents, signature } = preview.data
  return (
    <div className='flex flex-col gap-4'>
      <div className='rounded-md border p-4'>
        <div className='mb-2 text-sm font-medium'>明细 / Line items</div>
        <div className='flex flex-col gap-1'>
          {lines.map((l) => (
            <div key={l.fee_item} className='flex items-center justify-between text-sm'>
              <span>{l.fee_item}</span>
              <span className='font-mono'>¥{(l.amount_cny_cents / 100).toFixed(2)}</span>
            </div>
          ))}
          <Separator className='my-2' />
          <div className='flex items-center justify-between font-medium'>
            <span>合计 / Total</span>
            <span className='font-mono'>¥{(total_cny_cents / 100).toFixed(2)}</span>
          </div>
        </div>
        <div className='mt-3 text-xs text-muted-foreground font-mono'>
          签名 / Signature: {signature.slice(0, 12)}…
        </div>
      </div>
      <div className='flex justify-end gap-2'>
        <Button variant='outline' disabled={!canSubmit} onClick={saveDraft}>
          {isEdit ? '保存修改' : '保存草稿'}
        </Button>
        <Button disabled={!canSubmit} onClick={saveAndSubmit}>
          保存并提交
        </Button>
      </div>
    </div>
  )
}
