import { useEffect } from 'react'
import { useParams } from '@tanstack/react-router'
import { useCustomer } from '@/features/customers/hooks'
import { useCountries } from '@/features/catalog/hooks/use-countries'
import { useQuotation } from './hooks'
import { QuotationSnapshotView } from './components/quotation-snapshot'

// QuotationPrint renders a print-optimized page with no app shell.
// Users cmd+P / ctrl+P and "Save as PDF" from the browser dialog.
// The page auto-triggers window.print on mount (once data is ready)
// so the user sees the print dialog immediately.
export function QuotationPrint() {
  const params = useParams({ strict: false }) as { id: string }
  const { data: q, isLoading } = useQuotation(params.id)
  const { data: customer } = useCustomer(q?.customer_id ?? '')
  const { data: countries } = useCountries(true)
  const country = countries?.find((c) => c.id === q?.country_id)

  useEffect(() => {
    // Defer until customer + country are resolved so the dialog shows
    // human-readable names rather than bare UUIDs.
    if (q && customer && country) {
      const t = setTimeout(() => window.print(), 200)
      return () => clearTimeout(t)
    }
  }, [q, customer, country])

  if (isLoading) return <p className='p-8'>加载中…</p>
  if (!q) return <p className='p-8'>未找到报价</p>

  return (
    <div className='mx-auto max-w-3xl bg-white p-8 text-black print:p-0'>
      <style>{`
        @media print {
          @page { size: A4; margin: 16mm; }
          body { background: white !important; }
        }
      `}</style>
      <header className='mb-6 border-b pb-3'>
        <h1 className='text-2xl font-bold'>报价书 / Quotation</h1>
        <p className='text-sm text-gray-600'>编号 / No.: {q.id.slice(0, 8)}</p>
      </header>
      <section className='mb-6'>
        <h2 className='mb-2 text-lg font-semibold'>1. 基本信息 / Basic Info</h2>
        <dl className='grid grid-cols-4 gap-1 text-sm'>
          <dt className='col-span-1 text-gray-600'>客户 / Customer</dt>
          <dd className='col-span-3'>{customer?.name ?? q.customer_id}</dd>
          <dt className='col-span-1 text-gray-600'>国家 / Country</dt>
          <dd className='col-span-3'>
            {country ? `${country.code} ${country.name_zh} / ${country.name_en}` : q.country_id}
          </dd>
          <dt className='col-span-1 text-gray-600'>服务级别 / Tier</dt>
          <dd className='col-span-3'>{q.service_tier}</dd>
          <dt className='col-span-1 text-gray-600'>状态 / Status</dt>
          <dd className='col-span-3'>{q.status}</dd>
          {q.submitted_at && (
            <>
              <dt className='col-span-1 text-gray-600'>提交 / Submitted</dt>
              <dd className='col-span-3'>{new Date(q.submitted_at).toLocaleString()}</dd>
            </>
          )}
          {q.reviewed_at && (
            <>
              <dt className='col-span-1 text-gray-600'>审核 / Reviewed</dt>
              <dd className='col-span-3'>{new Date(q.reviewed_at).toLocaleString()}</dd>
            </>
          )}
          {q.notes && (
            <>
              <dt className='col-span-1 text-gray-600'>备注 / Notes</dt>
              <dd className='col-span-3'>{q.notes}</dd>
            </>
          )}
        </dl>
      </section>
      {q.snapshot && (
        <section className='mb-6'>
          <h2 className='mb-2 text-lg font-semibold'>2. 明细 / Breakdown</h2>
          <QuotationSnapshotView snapshot={q.snapshot} />
        </section>
      )}
      <footer className='mt-8 text-xs text-gray-500'>
        —— 本文档由系统自动生成 / Auto-generated document ——
      </footer>
    </div>
  )
}
