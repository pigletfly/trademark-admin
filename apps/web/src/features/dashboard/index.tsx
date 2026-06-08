import { useMemo } from 'react'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import {
  QUOTATION_STATUS_LABEL_ZH,
  type QuotationStatus,
} from '@/features/quotation/types'
import { useDashboardSummary } from './hooks/use-dashboard'
import { KPICard } from './components/kpi-card'
import { RecentQuotations } from './components/recent-quotations'

function formatCNY(cents: number): string {
  return '¥' + (cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export function Dashboard() {
  const { data, isLoading } = useDashboardSummary()

  const statusMap = useMemo(() => {
    const m: Partial<Record<QuotationStatus, number>> = {}
    for (const row of data?.quotations_by_status ?? []) {
      m[row.status] = row.count
    }
    return m
  }, [data])

  const scopeLabel = data?.scope === 'self' ? '我的' : '全公司'

  return (
    <>
      <Header>
        <div className='me-auto text-lg font-semibold'>仪表盘</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div className='flex items-baseline justify-between'>
          <h1 className='text-2xl font-bold tracking-tight'>仪表盘</h1>
          <p className='text-sm text-muted-foreground'>范围：{scopeLabel}</p>
        </div>

        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}

        {data && (
          <>
            <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
              <KPICard
                title={`${scopeLabel}报价总数`}
                value={
                  (data.quotations_by_status ?? []).reduce((s, r) => s + r.count, 0).toLocaleString()
                }
                caption='包括所有状态'
              />
              <KPICard
                title={QUOTATION_STATUS_LABEL_ZH.submitted}
                value={(statusMap.submitted ?? 0).toLocaleString()}
                caption='待审核'
              />
              <KPICard
                title={QUOTATION_STATUS_LABEL_ZH.approved}
                value={(statusMap.approved ?? 0).toLocaleString()}
                caption={`合计金额：${formatCNY(data.approved_total_cny_cents)}`}
              />
              <KPICard
                title='30 天新客户'
                value={(data.new_customers_last_30_days ?? 0).toLocaleString()}
                caption='过去 30 天内创建'
              />
            </div>

            <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
              <Card>
                <CardHeader>
                  <CardTitle>状态分布 / Status Breakdown</CardTitle>
                </CardHeader>
                <CardContent>
                  <ul className='space-y-1 text-sm'>
                    {(['draft', 'submitted', 'approved', 'rejected', 'cancelled'] as QuotationStatus[]).map(
                      (s) => (
                        <li key={s} className='flex items-center justify-between'>
                          <span className='text-muted-foreground'>
                            {QUOTATION_STATUS_LABEL_ZH[s]}
                          </span>
                          <span className='font-mono'>{(statusMap[s] ?? 0).toLocaleString()}</span>
                        </li>
                      ),
                    )}
                  </ul>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>近期报价 / Recent Quotations</CardTitle>
                </CardHeader>
                <CardContent>
                  <RecentQuotations items={data.recent_quotations ?? []} />
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </Main>
    </>
  )
}
