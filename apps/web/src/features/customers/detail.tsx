import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
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
import { useCustomer } from './hooks'
import { CustomerFormDialog } from './components/customer-form-dialog'

const route = getRouteApi('/_authenticated/customers/$id')

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='grid grid-cols-4 gap-2 py-1'>
      <dt className='col-span-1 text-sm text-muted-foreground'>{label}</dt>
      <dd className='col-span-3 text-sm'>{value || '—'}</dd>
    </div>
  )
}

export function CustomerDetail() {
  const { id } = route.useParams()
  const { data, isLoading } = useCustomer(id)
  const [editOpen, setEditOpen] = useState(false)

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/customers'>← 返回列表</Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        {data && (
          <Card>
            <CardHeader className='flex flex-row items-center justify-between'>
              <CardTitle className='text-2xl'>{data.name}</CardTitle>
              <Button onClick={() => setEditOpen(true)}>编辑</Button>
            </CardHeader>
            <CardContent>
              <dl className='divide-y'>
                <Field label='行业' value={data.industry} />
                <Field label='回头客户' value={data.is_returning ? '是' : '否'} />
                <Field label='价格敏感' value={data.price_sensitive ? '是' : '否'} />
                <Field label='联系人' value={data.contact_name} />
                <Field label='电话' value={data.contact_phone} />
                <Field label='邮箱' value={data.contact_email} />
                <Field label='备注' value={data.notes} />
                <Field label='创建时间' value={new Date(data.created_at).toLocaleString()} />
                <Field label='更新时间' value={new Date(data.updated_at).toLocaleString()} />
              </dl>
            </CardContent>
          </Card>
        )}
      </Main>
      {data && (
        <CustomerFormDialog mode='edit' open={editOpen} onOpenChange={setEditOpen} initial={data} />
      )}
    </>
  )
}
