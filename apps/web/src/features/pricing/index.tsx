import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useMe } from '@/features/auth/hooks'
import { useCountries } from '@/features/catalog/hooks'
import { usePricingList } from './hooks'
import { PricingMatrix } from './components/pricing-matrix'
import { PricingEntryDrawer } from './components/pricing-entry-drawer'
import { PricingHistorySheet } from './components/pricing-history-sheet'
import type { PricingEntry, ServiceTier } from './types'

const route = getRouteApi('/_authenticated/pricing/')

export function Pricing() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const me = useMe()
  const canEdit = me.data?.role === 'admin'

  const { data: countries = [] } = useCountries(false)
  const selected = search.country_id ?? countries[0]?.id ?? ''
  const { data: entries = [], isLoading } = usePricingList({ country_id: selected })

  const [editing, setEditing] = useState<{ feeItem: string; tier: ServiceTier; current?: PricingEntry } | null>(null)
  const [history, setHistory] = useState<{ feeItem: string; tier: ServiceTier } | null>(null)
  const [newFeeItem, setNewFeeItem] = useState('')

  const setCountry = (id: string) => navigate({ search: (s) => ({ ...s, country_id: id }), replace: false })

  const startNewEntry = () => {
    const fee = newFeeItem.trim()
    if (!fee) return
    setEditing({ feeItem: fee, tier: 'basic', current: undefined })
    setNewFeeItem('')
  }

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>定价管理</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>定价管理</h2>
            <p className='text-muted-foreground'>
              按国家 × 服务级别 × 费用项维护报价模板。管理员可编辑；审核员只读。
            </p>
          </div>
          <div className='flex items-center gap-2'>
            <Select value={selected} onValueChange={setCountry}>
              <SelectTrigger className='w-56'>
                <SelectValue placeholder='选择国家' />
              </SelectTrigger>
              <SelectContent>
                {countries.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name_zh} · {c.code}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        {canEdit && selected && (
          <div className='flex items-center gap-2'>
            <input
              className='rounded-md border px-3 py-1.5 text-sm'
              placeholder='新费用项名称（如 application_fee）'
              value={newFeeItem}
              onChange={(e) => setNewFeeItem(e.target.value)}
            />
            <Button size='sm' onClick={startNewEntry} disabled={!newFeeItem.trim()}>
              新增条目
            </Button>
          </div>
        )}

        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}

        {selected && (
          <PricingMatrix
            entries={entries}
            canEdit={canEdit}
            onEditCell={(feeItem, tier, current) => setEditing({ feeItem, tier, current })}
            onOpenHistory={(feeItem, tier) => setHistory({ feeItem, tier })}
          />
        )}
      </Main>

      {editing && selected && (
        <PricingEntryDrawer
          open={!!editing}
          onOpenChange={(o) => !o && setEditing(null)}
          countryId={selected}
          feeItem={editing.feeItem}
          serviceTier={editing.tier}
          current={editing.current}
        />
      )}

      {history && selected && (
        <PricingHistorySheet
          open={!!history}
          onOpenChange={(o) => !o && setHistory(null)}
          countryId={selected}
          feeItem={history.feeItem}
          serviceTier={history.tier}
        />
      )}
    </>
  )
}
