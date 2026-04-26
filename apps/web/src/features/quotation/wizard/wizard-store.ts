import { create, type StoreApi, type UseBoundStore } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

import type { Quotation, ServiceTier } from '../types'

// WizardDraft carries the 4 editable quotation fields plus a default
// tier. Kept as a flat shape so zustand patch calls are trivial.
export interface WizardDraft {
  customer_id: string
  country_id: string
  service_tier: ServiceTier
  notes: string
}

export interface WizardState {
  currentStep: 0 | 1 | 2 | 3 | 4
  draft: WizardDraft
  editingId: string | null

  setStep: (step: 0 | 1 | 2 | 3 | 4) => void
  patchDraft: (patch: Partial<WizardDraft>) => void
  reset: () => void
  loadForEdit: (id: string, serverDraft: Quotation) => void
}

const EMPTY_DRAFT: WizardDraft = {
  customer_id: '',
  country_id: '',
  service_tier: 'basic',
  notes: '',
}

// createWizardStore is user-scoped — each authenticated user gets their
// own localStorage slot so logging in as a different user never sees
// the previous user's draft. The caller (route component) constructs
// the store lazily via useWizardStore().
export function createWizardStore(userId: string): UseBoundStore<StoreApi<WizardState>> {
  const storageKey = `quotation-wizard-draft:${userId}`
  return create<WizardState>()(
    persist(
      (set) => ({
        currentStep: 0,
        draft: { ...EMPTY_DRAFT },
        editingId: null,
        setStep: (step) => set({ currentStep: step }),
        patchDraft: (patch) => set((s) => ({ draft: { ...s.draft, ...patch } })),
        reset: () => set({ currentStep: 0, draft: { ...EMPTY_DRAFT }, editingId: null }),
        loadForEdit: (id, q) =>
          set({
            editingId: id,
            currentStep: 0,
            draft: {
              customer_id: q.customer_id,
              country_id: q.country_id,
              service_tier: q.service_tier,
              notes: q.notes ?? '',
            },
          }),
      }),
      {
        name: storageKey,
        storage: createJSONStorage(() => localStorage),
        // Only persist draft + currentStep + editingId — those are the
        // "user's in-progress work". Method references don't persist.
        partialize: (s) => ({
          currentStep: s.currentStep,
          draft: s.draft,
          editingId: s.editingId,
        }),
      },
    ),
  )
}

// Step validators — exported because the wizard shell uses them to
// enable/disable the "Next" button, and tests assert them directly.

export function isStepCustomerValid(d: WizardDraft): boolean {
  return d.customer_id.length > 0
}

export function isStepCountryValid(d: WizardDraft): boolean {
  return d.country_id.length > 0
}

export function isStepTierValid(d: WizardDraft): boolean {
  return d.service_tier === 'basic' || d.service_tier === 'standard' || d.service_tier === 'premium'
}

// isStepNotesValid: notes is optional, so always valid. Kept for symmetry
// in the step indicator.
export function isStepNotesValid(_d: WizardDraft): boolean {
  return true
}

// hasNonEmptyDraft is the "resume banner" trigger — any user-typed
// content means we should ask before silently reusing the state.
export function hasNonEmptyDraft(d: WizardDraft): boolean {
  return d.customer_id.length > 0 || d.country_id.length > 0 || d.notes.length > 0
}
