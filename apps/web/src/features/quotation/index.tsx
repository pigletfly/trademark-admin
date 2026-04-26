import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useQuotationsList } from './hooks'
import { quotationColumns } from './components/quotations-columns'
import { QuotationsTable } from './components/quotations-table'
import type { QuotationStatus } from './types'
import { QUOTATION_STATUS_LABEL_ZH } from './types'

type QuotationsSearch = {
  status?: QuotationStatus
  page?: number
  page_size?: number
}

const STATUS_OPTIONS: { value: QuotationStatus | '__all__'; label: string }[] = [
  { value: '__all__', label: '全部状态' },
  ...(Object.keys(QUOTATION_STATUS_LABEL_ZH) as QuotationStatus[]).map((s) => ({
    value: s,
    label: QUOTATION_STATUS_LABEL_ZH[s],
  })),
]

export function Quotations() {
  const search = useSearch({ strict: false }) as QuotationsSearch
  const navigate = useNavigate()

  const query = {
    status: search.status,
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }
  const { data, isLoading } = useQuotationsList(query)

  const setSearch = (patch: Partial<QuotationsSearch>) =>
    navigate({
      search: ((old: QuotationsSearch) => ({ ...old, ...patch })) as never,
      replace: false,
    })

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>报价</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div className='flex items-center justify-between'>
          <h2 className='text-2xl font-bold'>报价列表</h2>
          <Button asChild>
            <Link to='/quotations/new'>新建报价</Link>
          </Button>
        </div>
        <div className='flex items-center gap-3'>
          <Select
            value={search.status ?? '__all__'}
            onValueChange={(v) =>
              setSearch({ status: v === '__all__' ? undefined : (v as QuotationStatus), page: 1 })
            }
          >
            <SelectTrigger className='w-48'>
              <SelectValue placeholder='全部状态' />
            </SelectTrigger>
            <SelectContent>
              {STATUS_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <QuotationsTable
          data={data?.items ?? []}
          columns={quotationColumns}
          isLoading={isLoading}
        />
      </Main>
    </>
  )
}
