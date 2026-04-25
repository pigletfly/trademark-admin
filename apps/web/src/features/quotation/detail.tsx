import { useState } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useQuotation, useQuotationHistory } from './hooks'
import { QuotationStatusBadge } from './components/quotation-status-badge'
import { QuotationSnapshotView } from './components/quotation-snapshot'
import { QuotationActionBar } from './components/quotation-action-bar'
import { QuotationHistoryTimeline } from './components/quotation-history-timeline'
import { QuotationFormSheet } from './components/quotation-form-sheet'

export function QuotationDetail() {
  const params = useParams({ strict: false }) as { id: string }
  const { data: q, isLoading } = useQuotation(params.id)
  const { data: history } = useQuotationHistory(params.id)
  const [editOpen, setEditOpen] = useState(false)

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
                  <span className='font-mono text-base'>{q.id.slice(0, 8)}</span>
                  <QuotationStatusBadge status={q.status} />
                </CardTitle>
                <QuotationActionBar quotation={q} onEditDraft={() => setEditOpen(true)} />
              </CardHeader>
              <CardContent className='grid gap-6 md:grid-cols-2'>
                <div className='space-y-2 text-sm'>
                  <div><span className='text-muted-foreground'>客户：</span>{q.customer_id}</div>
                  <div><span className='text-muted-foreground'>国家：</span>{q.country_id}</div>
                  <div><span className='text-muted-foreground'>服务级别：</span>{q.service_tier}</div>
                  <div><span className='text-muted-foreground'>备注：</span>{q.notes || '—'}</div>
                  <div><span className='text-muted-foreground'>提交时间：</span>
                    {q.submitted_at ? new Date(q.submitted_at).toLocaleString() : '—'}
                  </div>
                  <div><span className='text-muted-foreground'>审核时间：</span>
                    {q.reviewed_at ? new Date(q.reviewed_at).toLocaleString() : '—'}
                  </div>
                  {q.review_comment && (
                    <div><span className='text-muted-foreground'>审核备注：</span>{q.review_comment}</div>
                  )}
                </div>
                <div>
                  {q.snapshot ? (
                    <QuotationSnapshotView snapshot={q.snapshot} />
                  ) : (
                    <p className='text-sm text-muted-foreground'>草稿尚未提交，提交后将冻结定价快照。</p>
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

            <QuotationFormSheet open={editOpen} onOpenChange={setEditOpen} initial={q} />
          </>
        )}
      </Main>
    </>
  )
}
