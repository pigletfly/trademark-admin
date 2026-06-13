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
  madrid_country_ids: [],
  single_country_ids: [],
  nice_category_codes: [],
  agent_level: 'agent_a',
  info_sections: [],
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
    expect(s.draft.agent_level).toBe('agent_a')
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
      id: 'q1',
      customer_id: 'c1',
      country_id: 'co1',
      service_tier: 'premium',
      status: 'draft',
      notes: 'edit-me',
      created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z',
      updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    const s = store.getState()
    expect(s.editingId).toBe('q1')
    expect(s.draft.customer_id).toBe('c1')
    expect(s.draft.single_country_ids).toEqual(['co1'])
    expect(s.draft.agent_level).toBe('agent_b')
    expect(s.draft.notes).toBe('edit-me')
    expect(s.currentStep).toBe(0)
  })

  it('loadForEdit prefers extended form fields when present', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1',
      customer_id: 'c1',
      country_id: 'co1',
      country_ids: ['co1', 'co2'],
      madrid_country_ids: ['co1'],
      single_country_ids: ['co2'],
      nice_category_codes: [9, 35],
      agent_level: 'agent_b',
      service_tier: 'standard',
      status: 'draft',
      info_sections: ['acceptance_time', 'real_cases'],
      notes: 'edit-me',
      created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z',
      updated_at: '2026-04-26T00:00:00Z',
    }
    store.getState().loadForEdit('q1', q)
    expect(store.getState().draft).toEqual({
      customer_id: 'c1',
      madrid_country_ids: ['co1'],
      single_country_ids: ['co2'],
      nice_category_codes: [9, 35],
      agent_level: 'agent_b',
      info_sections: ['acceptance_time', 'real_cases'],
      notes: 'edit-me',
    })
  })

  it('loadForEdit maps null notes to empty string', () => {
    const store = createWizardStore(USER_A)
    const q: Quotation = {
      id: 'q1',
      customer_id: 'c1',
      country_id: 'co1',
      service_tier: 'basic',
      status: 'draft',
      notes: null,
      created_by: USER_A,
      created_at: '2026-04-26T00:00:00Z',
      updated_at: '2026-04-26T00:00:00Z',
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

  it('isStepCountryValid requires at least one country', () => {
    expect(isStepCountryValid({ ...empty, customer_id: 'c1' })).toBe(false)
    expect(
      isStepCountryValid({
        ...empty,
        customer_id: 'c1',
        madrid_country_ids: ['co1'],
      })
    ).toBe(true)
    expect(
      isStepCountryValid({
        ...empty,
        customer_id: 'c1',
        single_country_ids: ['co2'],
      })
    ).toBe(true)
  })

  it('isStepTierValid accepts supported agent levels', () => {
    expect(isStepTierValid({ ...empty, agent_level: 'agent_a' })).toBe(true)
    expect(isStepTierValid({ ...empty, agent_level: 'agent_b' })).toBe(true)
  })
})
