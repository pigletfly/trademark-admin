import { useState } from 'react'
import { createFileRoute, Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { useAuthStore } from '@/stores/auth-store'
import { getWizardStore, QuotationWizard, hasNonEmptyDraft } from '@/features/quotation/wizard/quotation-wizard'
import { ResumeBanner } from '@/features/quotation/wizard/resume-banner'

export function NewQuotationPage() {
  const userId = useAuthStore((s) => s.auth.user?.id) ?? ''
  const store = getWizardStore(userId)
  const draft = store((s) => s.draft)
  const editingId = store((s) => s.editingId)
  const reset = store.getState().reset

  // Banner shows only when:
  //   - mount: draft is non-empty AND not an edit-session leftover
  //   - user hasn't explicitly dismissed it this render
  const initiallyShow = hasNonEmptyDraft(draft) || editingId !== null
  const [showBanner, setShowBanner] = useState(initiallyShow)

  // If we're in new mode but the store still has a stale editingId from
  // a prior /edit visit, wipe it so we don't accidentally PATCH a
  // stranger's quotation. We do this eagerly on mount.
  if (editingId !== null) {
    reset()
  }

  return (
    <>
      <Header fixed>
        <Button asChild variant='ghost' size='sm' className='me-auto'>
          <Link to='/quotations'>← 返回列表</Link>
        </Button>
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main className='flex flex-col gap-4'>
        <h2 className='text-2xl font-bold'>新建报价</h2>
        {showBanner && (
          <ResumeBanner
            onContinue={() => setShowBanner(false)}
            onDiscard={() => {
              reset()
              setShowBanner(false)
            }}
          />
        )}
        <QuotationWizard mode='create' />
      </Main>
    </>
  )
}

export const Route = createFileRoute('/_authenticated/quotations/new')({
  component: NewQuotationPage,
})
