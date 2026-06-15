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
type MadridEditor = { mode: 'base' | 'country'; current?: MadridPricingEntry }

export function Pricing() {
  const search = useSearch({ strict: false }) as PricingSearch
  const navigate = useNavigate()
  const me = useMe()
  const canEdit = me.data?.role === 'admin'

  const { data: countries = [] } = useCountries(false)
  const selected = search.country_id ?? countries[0]?.id ?? ''
  const selectedCountry = countries.find((country) => country.id === selected)

  const madrid = useMadridPricingList({
    country_id: selected,
    include_base: true,
  })
  const singleClass = useSingleClassPricingList({ country_id: selected })

  const [madridEditor, setMadridEditor] = useState<MadridEditor | null>(null)
  const [singleEditor, setSingleEditor] =
    useState<SingleClassPricingEntry | null>(null)
  const [createSingleOpen, setCreateSingleOpen] = useState(false)

  const madridRows = useMemo(() => madrid.data ?? [], [madrid.data])
  const singleRow = singleClass.data?.[0]
  const madridBase = useMemo(
    () => madridRows.find((row) => row.is_base_fee),
    [madridRows]
  )
  const madridCountry = useMemo(
    () => madridRows.find((row) => !row.is_base_fee),
    [madridRows]
  )

  const setCountry = (id: string) =>
    navigate({
      search: ((old: PricingSearch) => ({ ...old, country_id: id })) as never,
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
          <Select value={selected} onValueChange={setCountry}>
            <SelectTrigger className='w-56'>
              <SelectValue placeholder='选择国家' />
            </SelectTrigger>
            <SelectContent>
              {countries.map((country) => (
                <SelectItem key={country.id} value={country.id}>
                  {country.name_zh} · {country.code}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <Tabs defaultValue='single' className='flex flex-col gap-4'>
          <TabsList className='w-fit'>
            <TabsTrigger value='single'>单一分类</TabsTrigger>
            <TabsTrigger value='madrid'>马德里</TabsTrigger>
          </TabsList>

          <TabsContent value='single' className='mt-0'>
            <section className='flex flex-col gap-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div>
                  <h3 className='text-lg font-semibold'>单一分类定价</h3>
                  <p className='text-sm text-muted-foreground'>
                    字段来自“全单一途径注册申请”sheet。
                  </p>
                </div>
                {canEdit && selected && (
                  <Button
                    size='sm'
                    onClick={() =>
                      singleRow
                        ? setSingleEditor(singleRow)
                        : setCreateSingleOpen(true)
                    }
                  >
                    <Pencil className='mr-2 h-4 w-4' />
                    {singleRow ? '修改单一分类定价' : '新增单一分类定价'}
                  </Button>
                )}
              </div>

              {singleClass.isLoading && (
                <p className='text-sm text-muted-foreground'>加载中…</p>
              )}
              {!singleClass.isLoading && !singleRow && (
                <p className='text-sm text-muted-foreground'>
                  该国家暂无单一分类定价。
                </p>
              )}
              {singleRow && <SingleClassTable row={singleRow} />}
            </section>
          </TabsContent>

          <TabsContent value='madrid' className='mt-0'>
            <section className='flex flex-col gap-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div>
                  <h3 className='text-lg font-semibold'>马德里定价</h3>
                  <p className='text-sm text-muted-foreground'>
                    包含基础注册费与所选国家/地区指定费。
                  </p>
                </div>
                {canEdit && selected && (
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      variant='outline'
                      size='sm'
                      onClick={() =>
                        setMadridEditor({ mode: 'base', current: madridBase })
                      }
                    >
                      <Pencil className='mr-2 h-4 w-4' />
                      {madridBase ? '修改基础费' : '新增基础费'}
                    </Button>
                    <Button
                      size='sm'
                      onClick={() =>
                        setMadridEditor({
                          mode: 'country',
                          current: madridCountry,
                        })
                      }
                    >
                      <Pencil className='mr-2 h-4 w-4' />
                      {madridCountry ? '修改国家费' : '新增国家费'}
                    </Button>
                  </div>
                )}
              </div>

              {madrid.isLoading && (
                <p className='text-sm text-muted-foreground'>加载中…</p>
              )}
              {!madrid.isLoading && madridRows.length === 0 && (
                <p className='text-sm text-muted-foreground'>
                  暂无马德里基础费或国家费。
                </p>
              )}
              {madridRows.length > 0 && (
                <MadridTable
                  rows={madridRows}
                  countryLabel={selectedCountry?.name_zh}
                />
              )}
            </section>
          </TabsContent>
        </Tabs>
      </Main>

      {selected && (
        <MadridPricingDrawer
          open={!!madridEditor}
          onOpenChange={(open) => !open && setMadridEditor(null)}
          countryId={selected}
          countries={countries}
          onSavedCountryID={setCountry}
          mode={madridEditor?.mode ?? 'country'}
          current={madridEditor?.current}
        />
      )}
      {selected && (
        <SingleClassPricingDrawer
          open={!!singleEditor || createSingleOpen}
          onOpenChange={(open) => {
            if (!open) {
              setSingleEditor(null)
              setCreateSingleOpen(false)
            }
          }}
          countryId={selected}
          countries={countries}
          onSavedCountryID={setCountry}
          current={singleEditor ?? undefined}
        />
      )}
    </>
  )
}

function SingleClassTable({ row }: { row: SingleClassPricingEntry }) {
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
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
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
          </TableRow>
        </TableBody>
      </Table>
    </div>
  )
}

function MadridTable({
  rows,
  countryLabel,
}: {
  rows: MadridPricingEntry[]
  countryLabel?: string
}) {
  return (
    <div className='overflow-hidden rounded-md border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>序号</TableHead>
            <TableHead>国家/地区</TableHead>
            <TableHead>官费（瑞士法郎）</TableHead>
            <TableHead>我所代理费（人民币）</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell>
                {row.is_base_fee ? '基础' : (row.sequence_no ?? '—')}
              </TableCell>
              <TableCell>
                <div className='font-medium'>
                  {row.country_area || countryLabel}
                </div>
                {row.is_base_fee && (
                  <div className='text-xs text-muted-foreground'>
                    基础注册费
                  </div>
                )}
              </TableCell>
              <TableCell>{formatCHF(row.official_fee_chf_cents)}</TableCell>
              <TableCell>{formatCNY(row.agency_fee_cny_cents)}</TableCell>
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
