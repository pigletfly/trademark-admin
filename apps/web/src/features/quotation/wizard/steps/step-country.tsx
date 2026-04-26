import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import type { WizardState } from '../wizard-store'

interface Props {
  state: WizardState
}

export function StepCountry({ state }: Props) {
  const { data } = useCountries()
  return (
    <div className='flex flex-col gap-3'>
      <div className='space-y-1.5'>
        <Label htmlFor='wizard-country'>国家 / Country</Label>
        <Select
          value={state.draft.country_id}
          onValueChange={(v) => state.patchDraft({ country_id: v })}
        >
          <SelectTrigger id='wizard-country' className='w-full'>
            <SelectValue placeholder='请选择国家' />
          </SelectTrigger>
          <SelectContent>
            {data?.map((c) => (
              <SelectItem key={c.id} value={c.id}>
                {c.name_zh}（{c.code}）
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
