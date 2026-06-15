import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { RotateCcw, X } from 'lucide-react'

interface Props {
  onContinue: () => void
  onDiscard: () => void
}

// ResumeBanner appears at the top of /quotations/new when localStorage
// already has a non-empty draft. The two buttons are 继续 (keep the draft)
// and 放弃 (reset the store). The banner itself doesn't own state —
// the parent component controls visibility by not rendering it after
// the user picks either option.
export function ResumeBanner({ onContinue, onDiscard }: Props) {
  return (
    <Alert className='mb-4'>
      <RotateCcw className='h-4 w-4' />
      <AlertTitle>检测到未完成的草稿</AlertTitle>
      <AlertDescription className='flex items-center justify-between gap-3'>
        <span>要继续上次的草稿，还是重新开始？</span>
        <div className='flex gap-2'>
          <Button size='sm' variant='outline' onClick={onDiscard}>
            <X className='mr-1 h-4 w-4' /> 放弃
          </Button>
          <Button size='sm' onClick={onContinue}>
            继续
          </Button>
        </div>
      </AlertDescription>
    </Alert>
  )
}
