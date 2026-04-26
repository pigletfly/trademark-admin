import { useMemo } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import {
  createWizardStore,
  isStepCustomerValid,
  isStepCountryValid,
  isStepTierValid,
  type WizardState,
} from './wizard-store'
import { StepCustomer } from './steps/step-customer'
import { StepCountry } from './steps/step-country'
import { StepTier } from './steps/step-tier'
import { StepNotes } from './steps/step-notes'
import { StepPreview } from './steps/step-preview'

// The wizard store is user-scoped, so we construct it lazily per-user
// and memoize by user id. A pool is cheaper than recreating across
// re-renders, and matches the "one user = one persisted slot" contract.
const stores: Record<string, ReturnType<typeof createWizardStore>> = {}
function useWizardStoreForUser(userId: string) {
  if (!stores[userId]) {
    stores[userId] = createWizardStore(userId)
  }
  return stores[userId]
}

interface Props {
  mode: 'create' | 'edit'
  // When mode==='edit', the outer route component is responsible for
  // calling loadForEdit() BEFORE rendering this component. In 'create'
  // mode the outer component is responsible for showing/hiding the
  // ResumeBanner and calling reset() if the user picks 放弃.
}

const STEPS: {
  key: string
  label: string
  isValid: (d: WizardState['draft']) => boolean
}[] = [
  { key: 'customer', label: '客户', isValid: isStepCustomerValid },
  { key: 'country', label: '国家', isValid: isStepCountryValid },
  { key: 'tier', label: '级别', isValid: isStepTierValid },
  { key: 'notes', label: '备注', isValid: () => true },
  { key: 'preview', label: '预览', isValid: () => true },
]

export function QuotationWizard({ mode }: Props) {
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const useStore = useWizardStoreForUser(userId)
  const state = useStore()

  const canProceed = useMemo(() => {
    // To move FROM step N to N+1 the step N's validator must pass.
    return STEPS[state.currentStep]?.isValid(state.draft) ?? true
  }, [state.currentStep, state.draft])

  const goNext = () => {
    if (canProceed && state.currentStep < 4) {
      state.setStep((state.currentStep + 1) as WizardState['currentStep'])
    }
  }
  const goBack = () => {
    if (state.currentStep > 0) {
      state.setStep((state.currentStep - 1) as WizardState['currentStep'])
    }
  }

  const stepContent = (() => {
    switch (state.currentStep) {
      case 0:
        return <StepCustomer state={state} />
      case 1:
        return <StepCountry state={state} />
      case 2:
        return <StepTier state={state} />
      case 3:
        return <StepNotes state={state} />
      case 4:
        return <StepPreview state={state} onExit={() => state.reset()} />
    }
  })()

  return (
    <div className='flex flex-col gap-4'>
      <StepIndicator current={state.currentStep} />
      <Card>
        <CardHeader>
          <CardTitle>
            {mode === 'edit' ? '编辑报价' : '新建报价'} — 第 {state.currentStep + 1} 步:{' '}
            {STEPS[state.currentStep].label}
          </CardTitle>
          <CardDescription>
            {state.currentStep < 4
              ? '填写以下字段后点"下一步"。数据会自动保存在本地。'
              : '确认信息无误后可保存草稿或直接提交审核。'}
          </CardDescription>
        </CardHeader>
        <CardContent>{stepContent}</CardContent>
      </Card>
      {state.currentStep < 4 && (
        <div className='flex justify-between'>
          <Button
            variant='ghost'
            disabled={state.currentStep === 0}
            onClick={goBack}
          >
            <ChevronLeft className='mr-1 h-4 w-4' /> 上一步
          </Button>
          <Button disabled={!canProceed} onClick={goNext}>
            下一步 <ChevronRight className='ml-1 h-4 w-4' />
          </Button>
        </div>
      )}
      {state.currentStep === 4 && (
        <div className='flex justify-start'>
          <Button variant='ghost' onClick={goBack}>
            <ChevronLeft className='mr-1 h-4 w-4' /> 上一步
          </Button>
        </div>
      )}
    </div>
  )
}

// StepIndicator renders a dotted pill-strip "0/1/2/3/4" with the
// current step highlighted. No shadcn Stepper primitive exists; this
// is the minimal visual affordance — a dot per step.
function StepIndicator({ current }: { current: number }) {
  return (
    <div className='flex items-center gap-2'>
      {STEPS.map((s, i) => (
        <div key={s.key} className='flex items-center gap-2'>
          <div
            className={cn(
              'flex h-8 w-8 items-center justify-center rounded-full border text-xs font-medium',
              i < current && 'bg-primary text-primary-foreground border-primary',
              i === current && 'border-primary text-primary',
              i > current && 'text-muted-foreground',
            )}
            aria-current={i === current ? 'step' : undefined}
          >
            {i + 1}
          </div>
          <span
            className={cn(
              'text-sm',
              i === current ? 'font-medium' : 'text-muted-foreground',
            )}
          >
            {s.label}
          </span>
          {i < STEPS.length - 1 && <div className='mx-1 h-px w-6 bg-border' />}
        </div>
      ))}
    </div>
  )
}

// Re-export a convenience hook so routes don't need to import createWizardStore
// directly — they use getStore(userId).reset() for mode transitions.
export function getWizardStore(userId: string) {
  return useWizardStoreForUser(userId)
}

// __resetWizardStorePool is for tests ONLY. Vitest runs multiple
// scenarios in one process; localStorage.clear() between tests does
// NOT clear the in-memory stores cached in `stores`. Call this in
// beforeEach to guarantee a clean slate.
export function __resetWizardStorePool() {
  for (const k of Object.keys(stores)) {
    stores[k].getState().reset()
    delete stores[k]
  }
}

// hasNonEmptyDraft / reset etc. are re-exported from the store module
// for routes that do resume-banner handling.
export { hasNonEmptyDraft } from './wizard-store'
