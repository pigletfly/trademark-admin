import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import type { Quotation } from '../types'

interface Props {
  quotation: Quotation
}

// Only shown when status === 'approved'. Word downloads trigger the
// backend /export.docx endpoint directly via <a href> so the browser
// streams the bytes and fires the Save dialog. PDF is handled by the
// print route + browser print dialog — no server PDF pipeline.
export function QuotationExportActions({ quotation }: Props) {
  const navigate = useNavigate()
  if (quotation.status !== 'approved') return null

  const downloadWord = () => {
    // Using window.location so cookies ride along (CSRF + auth).
    const url = `/api/v1/quotations/${quotation.id}/export.docx`
    window.location.href = url
  }
  const openPrint = () => {
    navigate({ to: '/quotations/$id/print', params: { id: quotation.id } })
      .catch(() => toast.error('无法打开打印预览'))
  }
  return (
    <div className='flex gap-2'>
      <Button variant='outline' onClick={downloadWord}>导出 Word</Button>
      <Button variant='outline' onClick={openPrint}>打印预览 / PDF</Button>
    </div>
  )
}
