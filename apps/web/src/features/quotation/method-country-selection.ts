import type { Quotation, RegistrationMethod } from './types'

export interface MethodCountrySelection {
  madrid_country_ids: string[]
  single_country_ids: string[]
}

const METHOD_ORDER: RegistrationMethod[] = ['madrid', 'single']

export function deriveMethodCountrySelectionFromQuotation(
  quotation: Pick<
    Quotation,
    | 'country_id'
    | 'country_ids'
    | 'registration_methods'
    | 'madrid_country_ids'
    | 'single_country_ids'
  >
): MethodCountrySelection {
  if (
    quotation.madrid_country_ids?.length ||
    quotation.single_country_ids?.length
  ) {
    return {
      madrid_country_ids: quotation.madrid_country_ids ?? [],
      single_country_ids: quotation.single_country_ids ?? [],
    }
  }

  const countryIds = quotation.country_ids?.length
    ? quotation.country_ids
    : quotation.country_id
      ? [quotation.country_id]
      : []
  const methods =
    quotation.registration_methods?.length
      ? quotation.registration_methods
      : ['single']

  return {
    madrid_country_ids: methods.includes('madrid') ? countryIds : [],
    single_country_ids: methods.includes('single') ? countryIds : [],
  }
}

export function hasSelectedCountries(selection: MethodCountrySelection) {
  return (
    selection.madrid_country_ids.length > 0 ||
    selection.single_country_ids.length > 0
  )
}

export function selectedRegistrationMethods(
  selection: MethodCountrySelection
): RegistrationMethod[] {
  return METHOD_ORDER.filter((method) =>
    method === 'madrid'
      ? selection.madrid_country_ids.length > 0
      : selection.single_country_ids.length > 0
  )
}

export function uniqueCountryIds(selection: MethodCountrySelection): string[] {
  const out: string[] = []
  for (const countryId of [
    ...selection.madrid_country_ids,
    ...selection.single_country_ids,
  ]) {
    if (!out.includes(countryId)) {
      out.push(countryId)
    }
  }
  return out
}
