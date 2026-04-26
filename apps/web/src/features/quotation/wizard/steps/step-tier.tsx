import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import type { ServiceTier } from '../../types'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

const OPTIONS: { value: ServiceTier; zh: string; desc: string }[] = [
  { value: 'basic', zh: '基础', desc: '标准申请流程' },
  { value: 'standard', zh: '标准', desc: '含审查反馈跟进' },
  { value: 'premium', zh: '尊享', desc: '全流程专员支持' },
]

export function StepTier({ state }: Props) {
  return (
    <div className='flex flex-col gap-3'>
      <Label>服务级别 / Service Tier</Label>
      <RadioGroup
        value={state.draft.service_tier}
        onValueChange={(v) => state.patchDraft({ service_tier: v as ServiceTier })}
        className='flex flex-col gap-3'
      >
        {OPTIONS.map((o) => (
          <label
            key={o.value}
            className='flex items-start gap-3 rounded-md border p-3 cursor-pointer hover:bg-muted/40'
          >
            <RadioGroupItem value={o.value} id={`tier-${o.value}`} />
            <div className='flex flex-col'>
              <span className='font-medium'>{o.zh} ({o.value})</span>
              <span className='text-xs text-muted-foreground'>{o.desc}</span>
            </div>
          </label>
        ))}
      </RadioGroup>
    </div>
  )
}
