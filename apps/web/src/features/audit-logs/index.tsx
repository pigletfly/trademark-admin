import { useNavigate, useSearch } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useAuditLogsList } from './hooks'
import { AuditLogsTable } from './components/audit-logs-table'

type AuditSearch = {
  resource_type?: string
  user_id?: string
  page?: number
  page_size?: number
}

export function AuditLogs() {
  const search = useSearch({ strict: false }) as AuditSearch
  const navigate = useNavigate()
  const query = {
    resource_type: search.resource_type ?? '',
    user_id: search.user_id ?? '',
    page: search.page ?? 1,
    page_size: search.page_size ?? 20,
  }
  const { data, isLoading } = useAuditLogsList(query)

  const setSearch = (patch: Partial<AuditSearch>) =>
    navigate({
      search: ((old: AuditSearch) => ({ ...old, ...patch })) as never,
      replace: false,
    })

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>审计日志</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-1 flex-col gap-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>审计日志</h2>
          <p className='text-muted-foreground'>
            管理员专属：按资源类型或操作人查看所有写操作的历史记录。
          </p>
        </div>
        <AuditLogsTable
          data={data?.items ?? []}
          total={data?.total ?? 0}
          page={query.page}
          pageSize={query.page_size}
          resourceType={query.resource_type}
          userId={query.user_id}
          onResourceTypeChange={(v) =>
            setSearch({ resource_type: v || undefined, page: 1 })
          }
          onUserIdChange={(v) => setSearch({ user_id: v || undefined, page: 1 })}
          onPageChange={(page) => setSearch({ page })}
        />
        {isLoading && <p className='text-sm text-muted-foreground'>正在加载…</p>}
      </Main>
    </>
  )
}
