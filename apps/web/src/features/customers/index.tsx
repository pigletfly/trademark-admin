import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useCustomersList } from './hooks'
import { CustomersTable } from './components/customers-table'
import { CustomerFormDialog } from './components/customer-form-dialog'

const route = getRouteApi('/_authenticated/customers/')

export function Customers() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const query = {
    q: search.q ?? '',
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }
  const { data, isLoading } = useCustomersList(query)
  const [createOpen, setCreateOpen] = useState(false)

  const setSearch = (patch: Partial<typeof search>) =>
    navigate({ search: (old) => ({ ...old, ...patch }), replace: false })

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>客户档案</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-1 flex-col gap-4'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>客户档案</h2>
            <p className='text-muted-foreground'>
              按角色可见：业务员只看自建，国际部商务与管理员看全部。
            </p>
          </div>
          <Button onClick={() => setCreateOpen(true)}>新建客户</Button>
        </div>
        <CustomersTable
          data={data?.items ?? []}
          total={data?.total ?? 0}
          page={query.page}
          pageSize={query.page_size}
          queryText={query.q}
          onQueryChange={(q) => setSearch({ q: q || undefined, page: 1 })}
          onPageChange={(page) => setSearch({ page })}
        />
        {isLoading && <p className='text-sm text-muted-foreground'>正在加载…</p>}
      </Main>
      <CustomerFormDialog mode='create' open={createOpen} onOpenChange={setCreateOpen} />
    </>
  )
}
