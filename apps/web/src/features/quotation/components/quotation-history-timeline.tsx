import {
  QUOTATION_STATUS_LABEL_ZH,
  type QuotationHistoryEntry,
} from '../types'

interface Props {
  items: QuotationHistoryEntry[]
}

export function QuotationHistoryTimeline({ items }: Props) {
  if (items.length === 0) {
    return <p className='text-sm text-muted-foreground'>暂无状态变更记录。</p>
  }
  return (
    <ol className='relative ms-2 space-y-4 border-s ps-4'>
      {items.map((e, idx) => (
        <li key={idx} className='relative'>
          <span className='absolute -start-[7px] top-1.5 h-3 w-3 rounded-full bg-primary' />
          <div className='text-sm'>
            <span className='font-medium'>
              {QUOTATION_STATUS_LABEL_ZH[e.from_status]} → {QUOTATION_STATUS_LABEL_ZH[e.to_status]}
            </span>
            <span className='ms-2 text-xs text-muted-foreground'>
              {new Date(e.at).toLocaleString()}
            </span>
          </div>
          {e.comment && <p className='mt-1 text-sm text-muted-foreground'>{e.comment}</p>}
          {e.diff_json && (
            <p className='mt-1 text-xs text-muted-foreground'>
              调价：¥{(e.diff_json.total_before / 100).toFixed(2)} → ¥{(e.diff_json.total_after / 100).toFixed(2)}
            </p>
          )}
        </li>
      ))}
    </ol>
  )
}
