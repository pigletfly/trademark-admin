import { useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { Pencil } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useMe } from '@/features/auth/hooks'
import { useCountries } from '@/features/catalog/hooks'
import {
  MadridPricingDrawer,
  SingleClassPricingDrawer,
} from './components/method-pricing-drawers'
import { useMadridPricingList, useSingleClassPricingList } from './hooks'
import type { MadridPricingEntry, SingleClassPricingEntry } from './types'

type PricingSearch = { country_id?: string }
type MadridEditor =
  | { kind: 'base'; mode: 'create' }
  | { kind: 'base'; mode: 'edit'; current: MadridPricingEntry }
  | { kind: 'country'; mode: 'create'; lockedCountryId?: string }
  | { kind: 'country'; mode: 'edit'; current: MadridPricingEntry }
type SingleClassEditor =
  | { mode: 'create'; lockedCountryId?: string }
  | { mode: 'edit'; current: SingleClassPricingEntry }

const ALL_COUNTRIES = '__all__'

export function Pricing() {
  const search = useSearch({ strict: false }) as PricingSearch
  const navigate = useNavigate()
  const me = useMe()
  const canEdit = me.data?.role === 'admin'

  const { data: countries = [] } = useCountries(false)
  const selectedCountryId = search.country_id

  const madrid = useMadridPricingList({
    country_id: selectedCountryId,
    include_base: true,
  })
  const singleClass = useSingleClassPricingList({
    country_id: selectedCountryId,
  })

  const [madridEditor, setMadridEditor] = useState<MadridEditor | null>(null)
  const [singleEditor, setSingleEditor] = useState<SingleClassEditor | null>(
    null
  )

  const madridRows = useMemo(() => madrid.data ?? [], [madrid.data])
  const singleRows = useMemo(() => singleClass.data ?? [], [singleClass.data])
  const madridBaseRow = useMemo(
    () => madridRows.find((row) => row.is_base_fee),
    [madridRows]
  )
  const madridCountryRows = useMemo(
    () => madridRows.filter((row) => !row.is_base_fee),
    [madridRows]
  )

  const existingSingleCountryIds = useMemo(
    () => new Set(singleRows.map((row) => row.country_id)),
    [singleRows]
  )
  const singleCreateCountries = useMemo(() => {
    if (selectedCountryId) {
      const country = countries.find((item) => item.id === selectedCountryId)
      if (!country || existingSingleCountryIds.has(selectedCountryId)) {
        return []
      }
      return [country]
    }
    return countries.filter(
      (country) => !existingSingleCountryIds.has(country.id)
    )
  }, [countries, existingSingleCountryIds, selectedCountryId])
  const singleAddDisabledReason = useMemo(() => {
    if (!canEdit) return null
    if (selectedCountryId && existingSingleCountryIds.has(selectedCountryId)) {
      return '当前筛选国家已有生效定价，请使用行内修改。'
    }
    if (!selectedCountryId && singleCreateCountries.length === 0) {
      return '当前已无可新增国家。'
    }
    return null
  }, [
    canEdit,
    existingSingleCountryIds,
    selectedCountryId,
    singleCreateCountries.length,
  ])
  const madridMemberCountries = useMemo(
    () => countries.filter((country) => country.is_madrid_member),
    [countries]
  )
  const existingMadridCountryIds = useMemo(
    () =>
      new Set(
        madridCountryRows.flatMap((row) => (row.country_id ? [row.country_id] : []))
      ),
    [madridCountryRows]
  )
  const madridCreateCountries = useMemo(() => {
    if (selectedCountryId) {
      const country = madridMemberCountries.find(
        (item) => item.id === selectedCountryId
      )
      if (!country || existingMadridCountryIds.has(selectedCountryId)) {
        return []
      }
      return [country]
    }
    return madridMemberCountries.filter(
      (country) => !existingMadridCountryIds.has(country.id)
    )
  }, [existingMadridCountryIds, madridMemberCountries, selectedCountryId])
  const madridAddDisabledReason = useMemo(() => {
    if (!canEdit) return null
    if (selectedCountryId) {
      const selectedCountry = madridMemberCountries.find(
        (country) => country.id === selectedCountryId
      )
      if (!selectedCountry) {
        return '当前筛选国家不支持马德里申请。'
      }
      if (existingMadridCountryIds.has(selectedCountryId)) {
        return '当前筛选国家已有生效定价，请使用行内修改。'
      }
    }
    if (!selectedCountryId && madridCreateCountries.length === 0) {
      return '当前已无可新增国家。'
    }
    return null
  }, [
    canEdit,
    existingMadridCountryIds,
    madridCreateCountries.length,
    madridMemberCountries,
    selectedCountryId,
  ])

  const setCountry = (id?: string) =>
    navigate({
      search: ((old: PricingSearch) => {
        const next = { ...old }
        if (id) {
          next.country_id = id
        } else {
          delete next.country_id
        }
        return next
      }) as never,
      replace: false,
    })

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
              按注册方式维护马德里与单一分类报价，新建报价时按国家和注册方式自动关联。
            </p>
          </div>
        </div>

        <Tabs defaultValue='single' className='flex flex-col gap-4'>
          <TabsList className='w-fit'>
            <TabsTrigger value='single'>单一分类</TabsTrigger>
            <TabsTrigger value='madrid'>马德里</TabsTrigger>
          </TabsList>

          <TabsContent value='single' className='mt-0'>
            <section className='flex flex-col gap-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Select
                    value={selectedCountryId ?? ALL_COUNTRIES}
                    onValueChange={(value) =>
                      setCountry(
                        value === ALL_COUNTRIES ? undefined : value
                      )
                    }
                  >
                    <SelectTrigger
                      aria-label='单一分类国家筛选'
                      className='w-56'
                    >
                      <SelectValue placeholder='全部国家/地区' />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_COUNTRIES}>
                        全部国家/地区
                      </SelectItem>
                      {countries.map((country) => (
                        <SelectItem key={country.id} value={country.id}>
                          {country.name_zh} · {country.code}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <div>
                    <h3 className='text-lg font-semibold'>单一分类定价</h3>
                    <p className='text-sm text-muted-foreground'>
                      字段来自“全单一途径注册申请”sheet。
                    </p>
                  </div>
                </div>
                {canEdit && (
                  <div className='flex flex-col items-end gap-1'>
                    <Button
                      size='sm'
                      disabled={singleCreateCountries.length === 0}
                      onClick={() =>
                        setSingleEditor({
                          mode: 'create',
                          lockedCountryId: selectedCountryId,
                        })
                      }
                    >
                      <Pencil className='mr-2 h-4 w-4' />
                      新增单一分类定价
                    </Button>
                    {singleAddDisabledReason && (
                      <p className='text-sm text-muted-foreground'>
                        {singleAddDisabledReason}
                      </p>
                    )}
                  </div>
                )}
              </div>

              {singleClass.isLoading && (
                <p className='text-sm text-muted-foreground'>加载中…</p>
              )}
              {!singleClass.isLoading && singleRows.length === 0 && (
                <p className='text-sm text-muted-foreground'>
                  {selectedCountryId
                    ? '当前筛选国家暂无生效定价。'
                    : '暂无单一分类定价。'}
                </p>
              )}
              {singleRows.length > 0 && (
                <SingleClassTable
                  rows={singleRows}
                  canEdit={canEdit}
                  onEdit={(row) =>
                    setSingleEditor({ mode: 'edit', current: row })
                  }
                />
              )}
            </section>
          </TabsContent>

          <TabsContent value='madrid' className='mt-0'>
            <div className='flex flex-col gap-6'>
              <section className='flex flex-col gap-3'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div>
                    <h3 className='text-lg font-semibold'>马德里基础费</h3>
                    <p className='text-sm text-muted-foreground'>
                      基础注册费不受国家筛选影响。
                    </p>
                  </div>
                  {canEdit && !madridBaseRow && (
                    <Button
                      size='sm'
                      onClick={() =>
                        setMadridEditor({ kind: 'base', mode: 'create' })
                      }
                    >
                      <Pencil className='mr-2 h-4 w-4' />
                      新增基础费
                    </Button>
                  )}
                </div>

                {madrid.isLoading && (
                  <p className='text-sm text-muted-foreground'>加载中…</p>
                )}
                {!madrid.isLoading && !madridBaseRow && (
                  <p className='text-sm text-muted-foreground'>暂无基础费。</p>
                )}
                {madridBaseRow && (
                  <MadridBaseTable
                    row={madridBaseRow}
                    canEdit={canEdit}
                    onEdit={() =>
                      setMadridEditor({
                        kind: 'base',
                        mode: 'edit',
                        current: madridBaseRow,
                      })
                    }
                  />
                )}
              </section>

              <section className='flex flex-col gap-3'>
                <div>
                  <h3 className='text-lg font-semibold'>马德里国家费</h3>
                  <p className='text-sm text-muted-foreground'>
                    国家费支持按国家筛选、新增和行内修改。
                  </p>
                </div>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <Select
                    value={selectedCountryId ?? ALL_COUNTRIES}
                    onValueChange={(value) =>
                      setCountry(value === ALL_COUNTRIES ? undefined : value)
                    }
                  >
                    <SelectTrigger
                      aria-label='马德里国家费国家筛选'
                      className='w-56'
                    >
                      <SelectValue placeholder='全部国家/地区' />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={ALL_COUNTRIES}>
                        全部国家/地区
                      </SelectItem>
                      {countries.map((country) => (
                        <SelectItem key={country.id} value={country.id}>
                          {country.name_zh} · {country.code}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {canEdit && (
                    <div className='flex flex-col items-end gap-1'>
                      <Button
                        size='sm'
                        disabled={madridCreateCountries.length === 0}
                        onClick={() =>
                          setMadridEditor({
                            kind: 'country',
                            mode: 'create',
                            lockedCountryId: selectedCountryId,
                          })
                        }
                      >
                        <Pencil className='mr-2 h-4 w-4' />
                        新增马德里国家费
                      </Button>
                      {madridAddDisabledReason && (
                        <p className='text-sm text-muted-foreground'>
                          {madridAddDisabledReason}
                        </p>
                      )}
                    </div>
                  )}
                </div>

                {madrid.isLoading && (
                  <p className='text-sm text-muted-foreground'>加载中…</p>
                )}
                {!madrid.isLoading && madridCountryRows.length === 0 && (
                  <p className='text-sm text-muted-foreground'>
                    {selectedCountryId
                      ? '当前筛选国家暂无生效定价。'
                      : '暂无马德里国家费。'}
                  </p>
                )}
                {madridCountryRows.length > 0 && (
                  <MadridCountryTable
                    rows={madridCountryRows}
                    canEdit={canEdit}
                    onEdit={(row) =>
                      setMadridEditor({
                        kind: 'country',
                        mode: 'edit',
                        current: row,
                      })
                    }
                  />
                )}
              </section>
            </div>
          </TabsContent>
        </Tabs>
      </Main>

      {madridEditor && (
        <MadridPricingDrawer
          open={!!madridEditor}
          onOpenChange={(open) => !open && setMadridEditor(null)}
          kind={madridEditor.kind}
          mode={madridEditor.mode}
          lockedCountryId={
            madridEditor.kind === 'country' && madridEditor.mode === 'create'
              ? madridEditor.lockedCountryId
              : undefined
          }
          availableCountries={madridCreateCountries}
          countries={countries}
          onSavedCountryID={setCountry}
          current={madridEditor.mode === 'edit' ? madridEditor.current : undefined}
        />
      )}
      {singleEditor && (
        <SingleClassPricingDrawer
          open={!!singleEditor}
          onOpenChange={(open) => {
            if (!open) {
              setSingleEditor(null)
            }
          }}
          mode={singleEditor.mode}
          lockedCountryId={
            singleEditor.mode === 'create'
              ? singleEditor.lockedCountryId
              : undefined
          }
          availableCountries={singleCreateCountries}
          countries={countries}
          onSavedCountryID={setCountry}
          current={
            singleEditor.mode === 'edit' ? singleEditor.current : undefined
          }
        />
      )}
    </>
  )
}

function SingleClassTable({
  rows,
  canEdit,
  onEdit,
}: {
  rows: SingleClassPricingEntry[]
  canEdit: boolean
  onEdit: (row: SingleClassPricingEntry) => void
}) {
  return (
    <div className='overflow-hidden rounded-md border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>国家/地区</TableHead>
            <TableHead>首类（不含税）</TableHead>
            <TableHead>首类（含税 6%）</TableHead>
            <TableHead>首类（含税 1%）</TableHead>
            <TableHead>每次类（不含税）</TableHead>
            <TableHead>受理需时</TableHead>
            <TableHead>注册需时</TableHead>
            {canEdit && <TableHead>操作</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell>
                <div className='font-medium'>{row.country_area}</div>
                <div className='text-xs text-muted-foreground'>
                  {row.continent}
                </div>
              </TableCell>
              <TableCell>{formatCNY(row.first_class_fee_cny_cents)}</TableCell>
              <TableCell>
                {formatCNY(row.first_class_fee_tax6_cny_cents)}
              </TableCell>
              <TableCell>
                {formatCNY(row.first_class_fee_tax1_cny_cents)}
              </TableCell>
              <TableCell>
                {formatCNY(row.additional_class_fee_cny_cents)}
              </TableCell>
              <TableCell>{row.acceptance_time || '—'}</TableCell>
              <TableCell>{row.registration_months || '—'}</TableCell>
              {canEdit && (
                <TableCell>
                  <Button size='sm' variant='ghost' onClick={() => onEdit(row)}>
                    修改
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function MadridBaseTable({
  row,
  canEdit,
  onEdit,
}: {
  row: MadridPricingEntry
  canEdit: boolean
  onEdit: () => void
}) {
  return (
    <div className='overflow-hidden rounded-md border'>
      <Table aria-label='马德里基础费表格'>
        <TableHeader>
          <TableRow>
            <TableHead>名称</TableHead>
            <TableHead>官费（瑞士法郎）</TableHead>
            <TableHead>我所代理费（人民币）</TableHead>
            {canEdit && <TableHead>操作</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>
              <div className='font-medium'>基础注册费</div>
            </TableCell>
            <TableCell>{formatCHF(row.official_fee_chf_cents)}</TableCell>
            <TableCell>{formatCNY(row.agency_fee_cny_cents)}</TableCell>
            {canEdit && (
              <TableCell>
                <Button size='sm' variant='ghost' onClick={onEdit}>
                  修改
                </Button>
              </TableCell>
            )}
          </TableRow>
        </TableBody>
      </Table>
    </div>
  )
}

function MadridCountryTable({
  rows,
  canEdit,
  onEdit,
}: {
  rows: MadridPricingEntry[]
  canEdit: boolean
  onEdit: (row: MadridPricingEntry) => void
}) {
  return (
    <div className='overflow-hidden rounded-md border'>
      <Table aria-label='马德里国家费表格'>
        <TableHeader>
          <TableRow>
            <TableHead>序号</TableHead>
            <TableHead>国家/地区</TableHead>
            <TableHead>官费（瑞士法郎）</TableHead>
            <TableHead>我所代理费（人民币）</TableHead>
            {canEdit && <TableHead>操作</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell>{row.sequence_no ?? '—'}</TableCell>
              <TableCell>
                <div className='font-medium'>{row.country_area}</div>
              </TableCell>
              <TableCell>{formatCHF(row.official_fee_chf_cents)}</TableCell>
              <TableCell>{formatCNY(row.agency_fee_cny_cents)}</TableCell>
              {canEdit && (
                <TableCell>
                  <Button size='sm' variant='ghost' onClick={() => onEdit(row)}>
                    修改
                  </Button>
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
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

function formatCHF(cents: number): string {
  return (
    'CHF ' +
    (cents / 100).toLocaleString('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    })
  )
}
