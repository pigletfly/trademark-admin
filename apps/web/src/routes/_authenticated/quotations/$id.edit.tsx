import { useEffect } from 'react'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useQuotation, quotationDetailQueryOptions } from '@/features/quotation/hooks'
import { getWizardStore, QuotationWizard } from '@/features/quotation/wizard/quotation-wizard'

export function EditQuotationPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const store = getWizardStore(userId)

  const { data: quotation } = useQuotation(id)

  // Load server draft into store once, on first render where quotation
  // is available. The store's loadForEdit overwrites any localStorage
  // residue (new-mode draft or stale edit-mode state from another id).
  useEffect(() => {
    if (quotation) {
      store.getState().loadForEdit(id, quotation)
    }
  }, [quotation, id, store])

  // If status leaves 'draft' while the user has this page open (a
  // reviewer approved/rejected/adjusted in another tab), bounce to
  // the detail page — editing a non-draft would fail anyway.
  useEffect(() => {
    if (quotation && quotation.status !== 'draft') {
      toast.info('报价状态已变更,无法编辑')
      navigate({ to: '/quotations/$id', params: { id } })
    }
  }, [quotation, id, navigate])

  // On unmount, clear the store so going back to /quotations/new
  // doesn't see a "zombie" edit state.
  useEffect(() => {
    return () => {
      store.getState().reset()
    }
  }, [store])

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/quotations/$id' params={{ id }}>
            ← 返回详情
          </Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <h2 className='text-2xl font-bold'>编辑报价 — {quotation?.serial_no ?? id.slice(0, 8)}</h2>
        {quotation && quotation.status === 'draft' && <QuotationWizard mode='edit' />}
      </Main>
    </>
  )
}

export const Route = createFileRoute('/_authenticated/quotations/$id/edit')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(quotationDetailQueryOptions(params.id)),
  component: EditQuotationPage,
})
