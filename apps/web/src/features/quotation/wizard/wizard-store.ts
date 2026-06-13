import { create, type StoreApi, type UseBoundStore } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'
import {
  deriveMethodCountrySelectionFromQuotation,
  hasSelectedCountries,
} from '../method-country-selection'

import type {
  AgentLevel,
  QuoteInfoSection,
  Quotation,
  ServiceTier,
} from '../types'

// WizardDraft keeps the quotation form flat so zustand patch calls stay
// predictable and localStorage migrations remain cheap.
export interface WizardDraft {
  customer_id: string
  madrid_country_ids: string[]
  single_country_ids: string[]
  nice_category_codes: number[]
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
  madrid_country_ids: [],
  single_country_ids: [],
  nice_category_codes: [],
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

  const migratePersistedState = (persisted: unknown): Partial<WizardState> => {
    const state = (persisted ?? {}) as {
      currentStep?: WizardState['currentStep']
      editingId?: string | null
      draft?: Record<string, unknown>
    }
    const draft = (state.draft ?? {}) as Partial<WizardDraft> & {
      country_ids?: string[]
      registration_methods?: Quotation['registration_methods']
    }
    const methodSelection = deriveMethodCountrySelectionFromQuotation({
      country_id: draft.country_ids?.[0] ?? '',
      country_ids: draft.country_ids,
      registration_methods: draft.registration_methods,
      madrid_country_ids: draft.madrid_country_ids,
      single_country_ids: draft.single_country_ids,
    })
    return {
      currentStep: state.currentStep ?? 0,
      editingId: state.editingId ?? null,
      draft: {
        customer_id: draft.customer_id ?? '',
        madrid_country_ids: methodSelection.madrid_country_ids,
        single_country_ids: methodSelection.single_country_ids,
        nice_category_codes: draft.nice_category_codes ?? [],
        agent_level: draft.agent_level ?? 'agent_a',
        info_sections: draft.info_sections ?? [],
        notes: draft.notes ?? '',
      },
    }
  }

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
              ...deriveMethodCountrySelectionFromQuotation(q),
              nice_category_codes: q.nice_category_codes ?? [],
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
        version: 1,
        migrate: migratePersistedState,
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
  return hasSelectedCountries({
    madrid_country_ids: d.madrid_country_ids ?? [],
    single_country_ids: d.single_country_ids ?? [],
  })
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
  const madridCountryIDs = d.madrid_country_ids ?? []
  const singleCountryIDs = d.single_country_ids ?? []
  const niceCategoryCodes = d.nice_category_codes ?? []
  const agentLevel = d.agent_level ?? 'agent_a'
  const infoSections = d.info_sections ?? []
  return (
    d.customer_id.length > 0 ||
    madridCountryIDs.length > 0 ||
    singleCountryIDs.length > 0 ||
    niceCategoryCodes.length > 0 ||
    agentLevel !== 'agent_a' ||
    infoSections.length > 0 ||
    d.notes.length > 0
  )
}
