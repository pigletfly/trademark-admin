import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCustomersList } from '@/features/customers/hooks'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

// Step 1: select the customer. Reuses the existing customers list hook;
// for now a Select of the first 100 is fine. If in the future users
// need a searchable combobox, swap to cmdk's Command — layout is ready.
export function StepCustomer({ state }: Props) {
  const { data } = useCustomersList({ page: 1, page_size: 100 })
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-customer'>客户 / Customer</Label>
        <Select
          value={state.draft.customer_id}
          onValueChange={(v) => state.patchDraft({ customer_id: v })}
        >
          <SelectTrigger id='wizard-customer' className='w-full'>
            <SelectValue placeholder='请选择客户' />
          </SelectTrigger>
          <SelectContent>
            {data?.items.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
