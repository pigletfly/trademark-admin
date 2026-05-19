import { Link, useParams } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { customerDetailQueryOptions } from '@/features/customers/hooks'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import { useNiceCategories } from '@/features/catalog/hooks/use-nice-categories'
import { useQuotation, useQuotationHistory } from './hooks'
import { QuotationStatusBadge } from './components/quotation-status-badge'
import { QuotationSnapshotView } from './components/quotation-snapshot'
import { QuotationActionBar } from './components/quotation-action-bar'
import { QuotationExportActions } from './components/quotation-export-actions'
import { QuotationHistoryTimeline } from './components/quotation-history-timeline'

const REGISTRATION_METHOD_LABELS: Record<string, string> = {
  madrid: '马德里',
  single: '单一分类',
}

const AGENT_LEVEL_LABELS: Record<string, string> = {
  agent_a: 'A 代理',
  agent_b: 'B 代理',
}

const INFO_SECTION_LABELS: Record<string, string> = {
  acceptance_time: '受理需时',
  registration_time: '注册需时',
  required_documents: '所需资料',
  registration_method_intro: '注册方式介绍',
  real_cases: '真实案例',
}

export function QuotationDetail() {
  const params = useParams({ strict: false }) as { id: string }
  const { data: q, isLoading } = useQuotation(params.id)
  const { data: history } = useQuotationHistory(params.id)
  // Countries must include disabled so deprecated countries on historical
  // quotations still resolve to a name instead of falling back to the id.
  const { data: countries } = useCountries(true)
  const { data: niceCategories } = useNiceCategories()
  const { data: customer } = useQuery({
    ...customerDetailQueryOptions(q?.customer_id ?? ''),
    enabled: !!q?.customer_id,
  })
  const countryIds = q
    ? q.country_ids?.length
      ? q.country_ids
      : [q.country_id]
    : []
  const countryNames = countryIds.map((id) => {
    const country = countries?.find((c) => c.id === id)
    return country ? `${country.name_zh}（${country.code}）` : id
  })
  const niceLabels = (q?.nice_category_codes ?? []).map((code) => {
    const category = niceCategories?.find((item) => item.code === code)
    return category
      ? `第 ${category.code} 类 ${category.name_zh}`
      : `第 ${code} 类`
  })

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/quotations'>← 返回列表</Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        {q && (
          <>
            <Card>
              <CardHeader className='flex flex-row items-center justify-between'>
                <CardTitle className='flex items-center gap-3 text-2xl'>
                  <span className='font-mono text-base'>
                    {q.serial_no ?? q.id.slice(0, 8)}
                  </span>
                  <QuotationStatusBadge status={q.status} />
                </CardTitle>
                <div className='flex items-center gap-2'>
                  <QuotationActionBar quotation={q} />
                  <QuotationExportActions quotation={q} />
                </div>
              </CardHeader>
              <CardContent className='grid gap-6 md:grid-cols-2'>
                <div className='space-y-2 text-sm'>
                  <div>
                    <span className='text-muted-foreground'>客户：</span>
                    {customer?.name ?? q.customer_id}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>国家：</span>
                    {countryNames.length
                      ? countryNames.join('、')
                      : q.country_id}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>商标类别：</span>
                    {niceLabels.length ? niceLabels.join('、') : '—'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>注册方式：</span>
                    {(q.registration_methods ?? [])
                      .map(
                        (method) => REGISTRATION_METHOD_LABELS[method] ?? method
                      )
                      .join('、') || '—'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>代理级别：</span>
                    {AGENT_LEVEL_LABELS[q.agent_level ?? ''] ?? q.service_tier}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>其他信息：</span>
                    {(q.info_sections ?? [])
                      .map((section) => INFO_SECTION_LABELS[section] ?? section)
                      .join('、') || '—'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>备注：</span>
                    {q.notes || '—'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>提交时间：</span>
                    {q.submitted_at
                      ? new Date(q.submitted_at).toLocaleString()
                      : '—'}
                  </div>
                  <div>
                    <span className='text-muted-foreground'>审核时间：</span>
                    {q.reviewed_at
                      ? new Date(q.reviewed_at).toLocaleString()
                      : '—'}
                  </div>
                  {q.review_comment && (
                    <div>
                      <span className='text-muted-foreground'>审核备注：</span>
                      {q.review_comment}
                    </div>
                  )}
                </div>
                <div>
                  {q.snapshot ? (
                    <QuotationSnapshotView snapshot={q.snapshot} />
                  ) : (
                    <p className='text-sm text-muted-foreground'>
                      草稿尚未提交，提交后将冻结定价快照。
                    </p>
                  )}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='text-lg'>状态变更</CardTitle>
              </CardHeader>
              <CardContent>
                <QuotationHistoryTimeline items={history?.items ?? []} />
              </CardContent>
            </Card>
          </>
        )}
      </Main>
    </>
  )
}
