import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

export function StepNotes({ state }: Props) {
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-notes'>备注 / Notes（可选）</Label>
        <Textarea
          id='wizard-notes'
          rows={6}
          placeholder='商标描述、客户特别要求、提交给 reviewer 的补充说明…'
          value={state.draft.notes}
          onChange={(e) => state.patchDraft({ notes: e.target.value })}
        />
      </div>
    </div>
  )
}
