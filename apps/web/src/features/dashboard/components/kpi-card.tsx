import type { ReactNode } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

interface Props {
  title: string
  value: ReactNode
  caption?: string
  icon?: ReactNode
}

export function KPICard({ title, value, caption, icon }: Props) {
  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{title}</CardTitle>
        {icon}
      </CardHeader>
      <CardContent>
        <div className='text-2xl font-bold'>{value}</div>
        {caption && <p className='text-xs text-muted-foreground'>{caption}</p>}
      </CardContent>
    </Card>
  )
}
