import { useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useUsersList } from './hooks'
import { UsersTable } from './components/users-table'
import { UserFormDialog } from './components/user-form-dialog'
import type { UserRole } from './types'

type UsersSearch = {
  q?: string
  role?: UserRole | ''
  page?: number
  page_size?: number
}

export function Users() {
  const search = useSearch({ strict: false }) as UsersSearch
  const navigate = useNavigate()
  const query = {
    q: search.q ?? '',
    role: (search.role ?? '') as UserRole | '',
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }
  const { data, isLoading } = useUsersList(query)
  const [createOpen, setCreateOpen] = useState(false)

  const setSearch = (patch: Partial<UsersSearch>) =>
    navigate({
      search: ((old: UsersSearch) => ({ ...old, ...patch })) as never,
      replace: false,
    })

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>用户管理</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-1 flex-col gap-4'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>用户管理</h2>
            <p className='text-muted-foreground'>
              管理员专属：创建、编辑账号、分配角色、重置密码、启用或禁用。
            </p>
          </div>
          <Button onClick={() => setCreateOpen(true)}>新建用户</Button>
        </div>
        <UsersTable
          data={data?.items ?? []}
          total={data?.total ?? 0}
          page={query.page}
          pageSize={query.page_size}
          queryText={query.q}
          roleFilter={query.role}
          onQueryChange={(q) => setSearch({ q: q || undefined, page: 1 })}
          onRoleChange={(role) =>
            setSearch({ role: role || undefined, page: 1 })
          }
          onPageChange={(page) => setSearch({ page })}
        />
        {isLoading && <p className='text-sm text-muted-foreground'>正在加载…</p>}
      </Main>
      <UserFormDialog
        mode='create'
        open={createOpen}
        onOpenChange={setCreateOpen}
      />
    </>
  )
}
