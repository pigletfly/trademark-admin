import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import type { ServiceTier } from '../types'
import { SERVICE_TIER_LABEL_ZH } from '../types'
import { formatCNY } from './pricing-matrix'
import { usePricingHistory } from '../hooks'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  countryId: string
  feeItem: string
  serviceTier: ServiceTier
}

export function PricingHistorySheet({
  open,
  onOpenChange,
  countryId,
  feeItem,
  serviceTier,
}: Props) {
  const { data = [], isLoading } = usePricingHistory({
    country_id: countryId,
    service_tier: serviceTier,
    fee_item: feeItem,
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='w-full sm:max-w-lg overflow-y-auto'>
        <SheetHeader>
          <SheetTitle>
            价格历史 · {feeItem} · {SERVICE_TIER_LABEL_ZH[serviceTier]}
          </SheetTitle>
          <SheetDescription>
            每个版本对应一次"生效 / 废止"事件。
          </SheetDescription>
        </SheetHeader>
        <div className='p-4'>
          {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
          {!isLoading && data.length === 0 && (
            <p className='text-sm text-muted-foreground'>暂无历史记录。</p>
          )}
          <ol className='relative border-l border-muted-foreground/30 pl-4'>
            {data.map((row) => (
              <li key={row.id} className='mb-6 ms-2'>
                <span className='absolute -start-1.5 mt-1.5 inline-block h-3 w-3 rounded-full bg-primary' />
                <div className='flex items-center gap-2'>
                  <span className='font-medium'>{formatCNY(row.amount_cny_cents)}</span>
                  {!row.effective_to && <Badge>当前生效</Badge>}
                </div>
                <p className='text-sm text-muted-foreground'>
                  生效: {row.effective_from}
                  {row.effective_to && ` → 废止: ${row.effective_to}`}
                </p>
                {row.notes && <p className='text-xs mt-1'>{row.notes}</p>}
              </li>
            ))}
          </ol>
        </div>
      </SheetContent>
    </Sheet>
  )
}
