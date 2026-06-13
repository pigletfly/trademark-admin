import { describe, expect, it } from 'vitest'
import type { WizardDraft } from './wizard-store'
import { buildQuotationRequest } from './quotation-wizard'

describe('buildQuotationRequest', () => {
  it('derives grouped country payload from method selections', () => {
    const draft: WizardDraft = {
      customer_id: 'cust-1',
      madrid_country_ids: ['madrid-1'],
      single_country_ids: ['single-1', 'single-2'],
      nice_category_codes: [9, 35],
      agent_level: 'agent_a',
      info_sections: ['acceptance_time'],
      notes: '  urgent quote  ',
    }

    expect(buildQuotationRequest(draft)).toEqual({
      customer_id: 'cust-1',
      country_id: 'madrid-1',
      country_ids: ['madrid-1', 'single-1', 'single-2'],
      madrid_country_ids: ['madrid-1'],
      single_country_ids: ['single-1', 'single-2'],
      nice_category_codes: [9, 35],
      registration_methods: ['madrid', 'single'],
      agent_level: 'agent_a',
      service_tier: 'basic',
      info_sections: ['acceptance_time'],
      notes: 'urgent quote',
    })
  })

  it('drops blank notes and keeps single-only requests valid', () => {
    const draft: WizardDraft = {
      customer_id: 'cust-2',
      madrid_country_ids: [],
      single_country_ids: ['single-1'],
      nice_category_codes: [9],
      agent_level: 'agent_b',
      info_sections: [],
      notes: '   ',
    }

    expect(buildQuotationRequest(draft)).toEqual({
      customer_id: 'cust-2',
      country_id: 'single-1',
      country_ids: ['single-1'],
      madrid_country_ids: [],
      single_country_ids: ['single-1'],
      nice_category_codes: [9],
      registration_methods: ['single'],
      agent_level: 'agent_b',
      service_tier: 'standard',
      info_sections: [],
      notes: null,
    })
  })
})
