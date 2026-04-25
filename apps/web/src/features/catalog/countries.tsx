import { useState } from 'react'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import type { Country } from './types'
import { useCountries } from './hooks'
import { CountriesTable } from './components/countries-table'
import { CountryEditDrawer } from './components/country-edit-drawer'

export function CatalogCountries() {
  const { data = [], isLoading } = useCountries(true)
  const [edit, setEdit] = useState<Country | null>(null)

  return (
    <>
      <Header fixed>
        <div className='me-auto text-lg font-semibold'>国家字典</div>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>国家字典</h2>
          <p className='text-muted-foreground'>管理员可编辑国家的双语名称、默认时效、Madrid 身份等。</p>
        </div>
        {isLoading && <p className='text-sm text-muted-foreground'>加载中…</p>}
        <CountriesTable data={data} onEdit={(row) => setEdit(row)} />
      </Main>
      <CountryEditDrawer open={!!edit} onOpenChange={(o) => !o && setEdit(null)} country={edit} />
    </>
  )
}
