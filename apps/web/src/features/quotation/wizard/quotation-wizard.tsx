import { useMemo, type ReactNode } from 'react'
import type { AxiosError } from 'axios'
import { useNavigate } from '@tanstack/react-router'
import { AlertCircle, Loader2, Save, Send } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import { useNiceCategories } from '@/features/catalog/hooks/use-nice-categories'
import { useCustomersList } from '@/features/customers/hooks'
import {
  hasSelectedCountries,
  selectedRegistrationMethods,
  uniqueCountryIds,
} from '../method-country-selection'
import {
  useCreateAndSubmit,
  useCreateQuotation,
  useUpdateAndSubmit,
  useUpdateQuotationDraft,
} from '../hooks/use-quotation-mutations'
import type {
  AgentLevel,
  CreateQuotationRequest,
  QuoteInfoSection,
} from '../types'
import { CountryMultiSelect } from './country-multi-select'
import { usePreview } from './hooks/use-preview'
import { NiceClassMultiSelect } from './nice-class-multi-select'
import {
  createWizardStore,
  serviceTierForAgentLevel,
  type WizardState,
} from './wizard-store'

const stores: Record<string, ReturnType<typeof createWizardStore>> = {}

function getWizardStoreForUser(userId: string) {
  if (!stores[userId]) {
    stores[userId] = createWizardStore(userId)
  }
  return stores[userId]
}

interface Props {
  mode: 'create' | 'edit'
}

const AGENT_LEVEL_OPTIONS: {
  value: AgentLevel
  label: string
  description: string
}[] = [
  { value: 'agent_a', label: 'A 代理', description: 'Default agent tier' },
  { value: 'agent_b', label: 'B 代理', description: 'Alternate agent tier' },
]

const INFO_SECTION_OPTIONS: {
  value: QuoteInfoSection
  label: string
}[] = [
  { value: 'acceptance_time', label: '受理需时' },
  { value: 'registration_time', label: '注册需时' },
  { value: 'required_documents', label: '所需资料' },
  { value: 'registration_method_intro', label: '注册方式介绍' },
  { value: 'real_cases', label: '真实案例' },
]

export function QuotationWizard({ mode }: Props) {
  const navigate = useNavigate()
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const useStore = getWizardStoreForUser(userId)
  const state = useStore()
  const customers = useCustomersList({ page: 1, page_size: 100 })
  const countries = useCountries()
  const niceCategories = useNiceCategories()
  const createMut = useCreateQuotation()
  const createSubmitMut = useCreateAndSubmit()
  const updateMut = useUpdateQuotationDraft()
  const updateSubmitMut = useUpdateAndSubmit()

  const draft = normalizeDraft(state.draft)
  const methodCountrySelection = {
    madrid_country_ids: draft.madrid_country_ids,
    single_country_ids: draft.single_country_ids,
  }
  const countryIds = uniqueCountryIds(methodCountrySelection)
  const primaryCountryId = countryIds[0] ?? ''
  const registrationMethods =
    selectedRegistrationMethods(methodCountrySelection)
  const serviceTier = serviceTierForAgentLevel(draft.agent_level)
  const madridCountries = useMemo(
    () => (countries.data ?? []).filter((country) => country.is_madrid_member),
    [countries.data]
  )
  const preview = usePreview({
    customer_id: draft.customer_id,
    country_id: primaryCountryId,
    country_ids: countryIds,
    madrid_country_ids: draft.madrid_country_ids,
    single_country_ids: draft.single_country_ids,
    nice_category_codes: draft.nice_category_codes,
    registration_methods: registrationMethods,
    service_tier: serviceTier,
  })
  const isEdit = state.editingId !== null
  const busy =
    createMut.isPending ||
    createSubmitMut.isPending ||
    updateMut.isPending ||
    updateSubmitMut.isPending
  const isFormValid =
    draft.customer_id.length > 0 &&
    hasSelectedCountries(methodCountrySelection) &&
    draft.nice_category_codes.length > 0 &&
    registrationMethods.length > 0
  const canSave = isFormValid && preview.isSuccess && !busy

  const body = useMemo(() => buildQuotationRequest(draft), [draft])

  const saveDraft = async () => {
    if (isEdit && state.editingId) {
      await updateMut.mutateAsync({ id: state.editingId, body })
      state.reset()
      navigate({ to: '/quotations/$id', params: { id: state.editingId } })
      return
    }
    const q = await createMut.mutateAsync(body)
    state.reset()
    navigate({ to: '/quotations/$id', params: { id: q.id } })
  }

  const saveAndSubmit = async () => {
    if (isEdit && state.editingId) {
      const result = await updateSubmitMut.mutateAsync({
        id: state.editingId,
        body,
      })
      state.reset()
      navigate({ to: '/quotations/$id', params: { id: result.id } })
      return
    }
    const result = await createSubmitMut.mutateAsync(body)
    state.reset()
    navigate({ to: '/quotations/$id', params: { id: result.id } })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{mode === 'edit' ? '编辑报价' : '新建报价'}</CardTitle>
        <CardDescription>
          表单数据会自动保存在本地，保存前会计算当前定价。
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className='grid gap-6'
          onSubmit={(event) => event.preventDefault()}
        >
          <div className='grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]'>
            <div className='grid gap-5'>
              <section className='grid gap-4'>
                <div className='grid gap-2'>
                  <Label htmlFor='quotation-customer'>客户 / Customer</Label>
                  <Select
                    value={draft.customer_id}
                    onValueChange={(value) =>
                      state.patchDraft({ customer_id: value })
                    }
                  >
                    <SelectTrigger id='quotation-customer' className='w-full'>
                      <SelectValue placeholder='请选择客户' />
                    </SelectTrigger>
                    <SelectContent>
                      {customers.data?.items.map((customer) => (
                        <SelectItem key={customer.id} value={customer.id}>
                          {customer.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <section className='grid gap-3'>
                  <Label htmlFor='quotation-nice-classes'>
                    商标类别 / Nice Classes
                  </Label>
                  <NiceClassMultiSelect
                    id='quotation-nice-classes'
                    categories={niceCategories.data ?? []}
                    value={draft.nice_category_codes}
                    loading={niceCategories.isLoading}
                    onValueChange={(codes) =>
                      state.patchDraft({ nice_category_codes: codes })
                    }
                  />
                </section>

                <section className='grid gap-3'>
                  <Label htmlFor='quotation-madrid-countries'>
                    马德里注册 / Madrid Registration
                  </Label>
                  <CountryMultiSelect
                    id='quotation-madrid-countries'
                    ariaLabel='Madrid registration countries'
                    placeholder='Select Madrid countries'
                    countries={madridCountries}
                    value={draft.madrid_country_ids}
                    loading={countries.isLoading}
                    onValueChange={(ids) =>
                      state.patchDraft({ madrid_country_ids: ids })
                    }
                  />
                  <p className='text-xs text-muted-foreground'>
                    仅显示支持马德里注册的国家。
                  </p>
                </section>

                <section className='grid gap-3'>
                  <Label htmlFor='quotation-single-countries'>
                    单一注册 / Single Filing
                  </Label>
                  <CountryMultiSelect
                    id='quotation-single-countries'
                    ariaLabel='Single filing countries'
                    placeholder='Select single-filing countries'
                    countries={countries.data ?? []}
                    value={draft.single_country_ids}
                    loading={countries.isLoading}
                    onValueChange={(ids) =>
                      state.patchDraft({ single_country_ids: ids })
                    }
                  />
                </section>

                <section className='grid gap-3'>
                  <Label>代理级别 / Agent Level</Label>
                  <RadioGroup
                    value={draft.agent_level}
                    onValueChange={(value) =>
                      state.patchDraft({ agent_level: value as AgentLevel })
                    }
                    className='grid gap-2 sm:grid-cols-2'
                  >
                    {AGENT_LEVEL_OPTIONS.map((option) => (
                      <label
                        key={option.value}
                        className='flex cursor-pointer items-start gap-3 rounded-md border p-3 transition-colors hover:bg-muted/40'
                      >
                        <RadioGroupItem
                          value={option.value}
                          id={`agent-level-${option.value}`}
                        />
                        <span className='grid gap-0.5'>
                          <span className='font-medium'>{option.label}</span>
                          <span className='text-xs text-muted-foreground'>
                            {option.description}
                          </span>
                        </span>
                      </label>
                    ))}
                  </RadioGroup>
                </section>

                <MultiCheckSection title='其他信息 / Extra Sections'>
                  {INFO_SECTION_OPTIONS.map((option) => (
                    <CheckboxRow
                      key={option.value}
                      id={`info-section-${option.value}`}
                      checked={draft.info_sections.includes(option.value)}
                      label={option.label}
                      onCheckedChange={(checked) =>
                        state.patchDraft({
                          info_sections: toggleValue(
                            draft.info_sections,
                            option.value,
                            checked
                          ),
                        })
                      }
                    />
                  ))}
                </MultiCheckSection>

                <div className='grid gap-2'>
                  <Label htmlFor='quotation-notes'>备注 / Notes</Label>
                  <Textarea
                    id='quotation-notes'
                    value={draft.notes}
                    onChange={(event) =>
                      state.patchDraft({ notes: event.target.value })
                    }
                    placeholder='可填写客户偏好、内部说明或报价口径'
                    rows={4}
                  />
                </div>
              </section>
            </div>

            <aside className='lg:sticky lg:top-20 lg:self-start'>
              <PreviewPanel
                isFormValid={isFormValid}
                isEdit={isEdit}
                busy={busy}
                canSave={canSave}
                preview={preview}
                onRetry={() => preview.refetch()}
                onSaveDraft={saveDraft}
                onSaveAndSubmit={saveAndSubmit}
              />
            </aside>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}

function MultiCheckSection({
  title,
  className,
  children,
}: {
  title: string
  className?: string
  children: ReactNode
}) {
  return (
    <section className='grid gap-3'>
      <Label>{title}</Label>
      <div
        className={cn(
          'grid gap-2 rounded-md border p-3 sm:grid-cols-2',
          className
        )}
      >
        {children}
      </div>
    </section>
  )
}

function CheckboxRow({
  id,
  checked,
  label,
  description,
  onCheckedChange,
}: {
  id: string
  checked: boolean
  label: string
  description?: string
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <label
      htmlFor={id}
      className='flex min-h-11 cursor-pointer items-start gap-3 rounded-md p-2 transition-colors hover:bg-muted/50'
    >
      <Checkbox
        id={id}
        checked={checked}
        onCheckedChange={(value) => onCheckedChange(value === true)}
      />
      <span className='grid gap-0.5 text-sm leading-tight'>
        <span className='font-medium'>{label}</span>
        {description && (
          <span className='text-xs text-muted-foreground'>{description}</span>
        )}
      </span>
    </label>
  )
}

function PreviewPanel({
  isFormValid,
  isEdit,
  busy,
  canSave,
  preview,
  onRetry,
  onSaveDraft,
  onSaveAndSubmit,
}: {
  isFormValid: boolean
  isEdit: boolean
  busy: boolean
  canSave: boolean
  preview: ReturnType<typeof usePreview>
  onRetry: () => void
  onSaveDraft: () => void
  onSaveAndSubmit: () => void
}) {
  return (
    <div className='grid gap-4 rounded-md border bg-muted/20 p-4'>
      <div>
        <h3 className='font-medium'>报价预览 / Preview</h3>
        <p className='text-sm text-muted-foreground'>
          合计会随客户、国家、注册方式和代理级别自动更新。
        </p>
      </div>
      <Separator />
      {!isFormValid && (
        <p className='text-sm text-muted-foreground'>
          请选择客户、商标类别，且至少选择一种注册方式的国家。
        </p>
      )}
      {isFormValid && preview.isLoading && (
        <div className='flex items-center gap-2 text-sm text-muted-foreground'>
          <Loader2 className='h-4 w-4 animate-spin' /> 计算中 / Computing…
        </div>
      )}
      {isFormValid && preview.isError && (
        <Alert variant='destructive'>
          <AlertCircle className='h-4 w-4' />
          <AlertTitle>预览失败 / Preview failed</AlertTitle>
          <AlertDescription className='grid gap-3'>
            <span>{previewErrorMessage(preview.error)}</span>
            <Button type='button' variant='outline' size='sm' onClick={onRetry}>
              重试 / Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}
      {isFormValid && preview.isSuccess && (
        <div className='grid gap-2'>
          {preview.data.lines.map((line, index) => (
            <div
              key={`${line.fee_item}-${index}`}
              className='flex items-center justify-between gap-3 text-sm'
            >
              <span className='min-w-0 truncate'>{line.fee_item}</span>
              <span className='font-mono'>
                {formatCNY(line.amount_cny_cents)}
              </span>
            </div>
          ))}
          <Separator />
          <div className='flex items-center justify-between gap-3 font-medium'>
            <span>合计 / Total</span>
            <span className='font-mono'>
              {formatCNY(preview.data.total_cny_cents)}
            </span>
          </div>
          <p className='text-xs break-all text-muted-foreground'>
            签名：<code>{preview.data.signature.slice(0, 12)}…</code>
          </p>
        </div>
      )}
      <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-1'>
        <Button
          type='button'
          variant='outline'
          disabled={!canSave || busy}
          onClick={onSaveDraft}
        >
          <Save className='mr-2 h-4 w-4' />
          {isEdit ? '保存修改' : '保存草稿'}
        </Button>
        <Button
          type='button'
          disabled={!canSave || busy}
          onClick={onSaveAndSubmit}
        >
          <Send className='mr-2 h-4 w-4' />
          保存并提交
        </Button>
      </div>
    </div>
  )
}

function previewErrorMessage(error: unknown) {
  const code = (error as AxiosError<{ code?: string }>)?.response?.data?.code
  if (code === 'ERR_MISSING_PRICING')
    return '所选国家/代理级别暂无定价，请调整选择或联系管理员。'
  if (code === 'ERR_NOT_FOUND') return '客户不存在，请重新选择客户。'
  return '预览失败，请稍后重试。'
}

function toggleValue<T>(values: T[], value: T, checked: boolean): T[] {
  if (checked) {
    return values.includes(value) ? values : [...values, value]
  }
  return values.filter((item) => item !== value)
}

function normalizeDraft(draft: WizardState['draft']): WizardState['draft'] {
  return {
    ...draft,
    madrid_country_ids: draft.madrid_country_ids ?? [],
    single_country_ids: draft.single_country_ids ?? [],
    nice_category_codes: draft.nice_category_codes ?? [],
    agent_level: draft.agent_level ?? 'agent_a',
    info_sections: draft.info_sections ?? [],
  }
}

export function buildQuotationRequest(
  draft: WizardState['draft']
): CreateQuotationRequest {
  const methodCountrySelection = {
    madrid_country_ids: draft.madrid_country_ids ?? [],
    single_country_ids: draft.single_country_ids ?? [],
  }
  const countryIds = uniqueCountryIds(methodCountrySelection)
  return {
    customer_id: draft.customer_id,
    country_id: countryIds[0] ?? '',
    country_ids: countryIds,
    madrid_country_ids: methodCountrySelection.madrid_country_ids,
    single_country_ids: methodCountrySelection.single_country_ids,
    nice_category_codes: draft.nice_category_codes ?? [],
    registration_methods: selectedRegistrationMethods(methodCountrySelection),
    agent_level: draft.agent_level,
    service_tier: serviceTierForAgentLevel(draft.agent_level),
    info_sections: draft.info_sections,
    notes: draft.notes.trim() ? draft.notes.trim() : null,
  }
}

function formatCNY(cents: number): string {
  return (
    '¥' +
    (cents / 100).toLocaleString('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  )
}

export function getWizardStore(userId: string) {
  return getWizardStoreForUser(userId)
}

export function __resetWizardStorePool() {
  for (const k of Object.keys(stores)) {
    stores[k].getState().reset()
    delete stores[k]
  }
}

export { hasNonEmptyDraft } from './wizard-store'
