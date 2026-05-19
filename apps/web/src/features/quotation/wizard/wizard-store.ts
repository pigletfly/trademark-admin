import { create, type StoreApi, type UseBoundStore } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

import type {
  AgentLevel,
  QuoteInfoSection,
  Quotation,
  RegistrationMethod,
  ServiceTier,
} from '../types'

// WizardDraft keeps the quotation form flat so zustand patch calls stay
// predictable and localStorage migrations remain cheap.
export interface WizardDraft {
  customer_id: string
  country_ids: string[]
  nice_category_codes: number[]
  registration_methods: RegistrationMethod[]
  agent_level: AgentLevel
  info_sections: QuoteInfoSection[]
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
  country_ids: [],
  nice_category_codes: [],
  registration_methods: ['single'],
  agent_level: 'agent_a',
  info_sections: [],
  notes: '',
}

export function serviceTierForAgentLevel(agentLevel: AgentLevel): ServiceTier {
  return agentLevel === 'agent_b' ? 'standard' : 'basic'
}

function agentLevelForServiceTier(serviceTier: ServiceTier): AgentLevel {
  return serviceTier === 'standard' || serviceTier === 'premium'
    ? 'agent_b'
    : 'agent_a'
}

// createWizardStore is user-scoped — each authenticated user gets their
// own localStorage slot so logging in as a different user never sees
// the previous user's draft. The caller (route component) constructs
// the store lazily via useWizardStore().
export function createWizardStore(
  userId: string
): UseBoundStore<StoreApi<WizardState>> {
  const storageKey = `quotation-wizard-draft:${userId}`
  return create<WizardState>()(
    persist(
      (set) => ({
        currentStep: 0,
        draft: { ...EMPTY_DRAFT },
        editingId: null,
        setStep: (step) => set({ currentStep: step }),
        patchDraft: (patch) =>
          set((s) => ({ draft: { ...s.draft, ...patch } })),
        reset: () =>
          set({ currentStep: 0, draft: { ...EMPTY_DRAFT }, editingId: null }),
        loadForEdit: (id, q) =>
          set({
            editingId: id,
            currentStep: 0,
            draft: {
              customer_id: q.customer_id,
              country_ids: q.country_ids?.length
                ? q.country_ids
                : [q.country_id],
              nice_category_codes: q.nice_category_codes ?? [],
              registration_methods: q.registration_methods?.length
                ? q.registration_methods
                : ['single'],
              agent_level:
                q.agent_level ?? agentLevelForServiceTier(q.service_tier),
              info_sections: q.info_sections ?? [],
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
      }
    )
  )
}

// Step validators — exported because the wizard shell uses them to
// enable/disable the "Next" button, and tests assert them directly.

export function isStepCustomerValid(d: WizardDraft): boolean {
  return d.customer_id.length > 0
}

export function isStepCountryValid(d: WizardDraft): boolean {
  return (d.country_ids ?? []).length > 0
}

export function isStepTierValid(d: WizardDraft): boolean {
  return (
    (d.agent_level ?? 'agent_a') === 'agent_a' || d.agent_level === 'agent_b'
  )
}

// isStepNotesValid: notes is optional, so always valid. Kept for symmetry
// in the step indicator.
export function isStepNotesValid(_d: WizardDraft): boolean {
  return true
}

// hasNonEmptyDraft is the "resume banner" trigger — any user-typed
// content means we should ask before silently reusing the state.
export function hasNonEmptyDraft(d: WizardDraft): boolean {
  const countryIds = d.country_ids ?? []
  const niceCategoryCodes = d.nice_category_codes ?? []
  const registrationMethods = d.registration_methods ?? ['single']
  const agentLevel = d.agent_level ?? 'agent_a'
  const infoSections = d.info_sections ?? []
  return (
    d.customer_id.length > 0 ||
    countryIds.length > 0 ||
    niceCategoryCodes.length > 0 ||
    registrationMethods.length !== 1 ||
    registrationMethods[0] !== 'single' ||
    agentLevel !== 'agent_a' ||
    infoSections.length > 0 ||
    d.notes.length > 0
  )
}
