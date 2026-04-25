import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useNiceCategories } from './hooks'
import { NiceCategoriesTable } from './components/nice-categories-table'

export function CatalogNiceCategories() {
  const { data = [], isLoading } = useNiceCategories()
  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>尼斯分类</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>尼斯分类</h2>
          <p className='text-muted-foreground'>国际商标尼斯分类 45 项，仅查看。</p>
        </div>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        <NiceCategoriesTable data={data} />
      </Main>
    </>
  )
}
