import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  createWizardStore,
  isStepCustomerValid,
  isStepCountryValid,
  isStepTierValid,
  type WizardDraft,
} from './wizard-store'
import type { Quotation } from '../types'

const USER_A = '00000000-0000-0000-0000-00000000A001'
const USER_B = '00000000-0000-0000-0000-00000000B002'

const empty: WizardDraft = {
  customer_id: '',
  country_id: '',
  service_tier: 'basic',
  notes: '',
}

describe('wizard-store', () => {
  beforeEach(() => {
    localStorage.clear()
  })
  afterEach(() => {
    localStorage.clear()
  })

  it('starts empty with step 0 and no editingId', () => {
    const store = createWizardStore(USER_A)
    const s = store.getState()
    expect(s.currentStep).toBe(0)
    expect(s.editingId).toBeNull()
    expect(s.draft).toEqual(empty)
  })

  it('patchDraft merges fields and preserves the rest', () => {
    const store = createWizardStore(USER_A)
    store.getState().patchDraft({ customer_id: 'c1' })
    store.getState().patchDraft({ notes: 'hi' })
    const s = store.getState()
    expect(s.draft.customer_id).toBe('c1')
    expect(s.draft.notes).toBe('hi')
    expect(s.draft.service_tier).toBe('basic')
  })

  it('reset clears draft, step, and editingId', () => {
    const store = createWizardStore(USER_A)
    store.getState().patchDraft({ customer_id: 'c1', notes: 'x' })
    store.getState().setStep(2)
    store.getState().reset()
    const s = store.getState()
    expect(s.draft).toEqual(empty)
    expect(s.currentStep).toBe(0)
    expect(s.editingId).toBeNull()
  })

  it('loadForEdit sets editingId and fills draft from a Quotation', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1', customer_id: 'c1', country_id: 'co1', service_tier: 'premium',
      status: 'draft', notes: 'edit-me', created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z', updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    const s = store.getState()
    expect(s.editingId).toBe('q1')
    expect(s.draft.customer_id).toBe('c1')
    expect(s.draft.country_id).toBe('co1')
    expect(s.draft.service_tier).toBe('premium')
    expect(s.draft.notes).toBe('edit-me')
    expect(s.currentStep).toBe(0)
  })

  it('loadForEdit maps null notes to empty string', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1', customer_id: 'c1', country_id: 'co1', service_tier: 'basic',
      status: 'draft', notes: null, created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z', updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    expect(store.getState().draft.notes).toBe('')
  })

  it('different user ids have independent localStorage keys', () => {
    const a = createWizardStore(USER_A)
    a.getState().patchDraft({ customer_id: 'cA' })
    const b = createWizardStore(USER_B)
    expect(b.getState().draft.customer_id).toBe('')
    b.getState().patchDraft({ customer_id: 'cB' })
    // Re-opening USER_A's store should still see cA.
    const a2 = createWizardStore(USER_A)
    expect(a2.getState().draft.customer_id).toBe('cA')
  })

  it('isStepCustomerValid requires a uuid-ish non-empty customer_id', () => {
    expect(isStepCustomerValid({ ...empty })).toBe(false)
    expect(isStepCustomerValid({ ...empty, customer_id: 'c1' })).toBe(true)
  })

  it('isStepCountryValid requires a non-empty country_id', () => {
    expect(isStepCountryValid({ ...empty, customer_id: 'c1' })).toBe(false)
    expect(isStepCountryValid({ ...empty, customer_id: 'c1', country_id: 'co1' })).toBe(true)
  })

  it('isStepTierValid accepts any enum value', () => {
    expect(isStepTierValid({ ...empty, service_tier: 'basic' })).toBe(true)
    expect(isStepTierValid({ ...empty, service_tier: 'standard' })).toBe(true)
    expect(isStepTierValid({ ...empty, service_tier: 'premium' })).toBe(true)
  })
})
