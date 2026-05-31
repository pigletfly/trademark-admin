import { useMemo } from 'react'
import type { Country } from '@/features/catalog/types'
import {
  FilterableMultiSelect,
  type FilterableMultiSelectOption,
} from './filterable-multi-select'

interface CountryMultiSelectProps {
  id: string
  countries: Country[]
  value: string[]
  onValueChange: (value: string[]) => void
  loading?: boolean
}

export function CountryMultiSelect({
  id,
  countries,
  value,
  onValueChange,
  loading = false,
}: CountryMultiSelectProps) {
  const options = useMemo(
    () =>
      countries.map(
        (country): FilterableMultiSelectOption<string> => ({
          value: country.id,
          triggerLabel: formatTriggerLabel(country),
          optionTitle: formatOptionTitle(country),
          optionDescription: country.name_en,
          accessibleLabel: formatOptionLabel(country),
          searchText: [country.code, country.name_zh, country.name_en].join(
            ' '
          ),
        })
      ),
    [countries]
  )

  return (
    <FilterableMultiSelect
      id={id}
      ariaLabel='Countries'
      placeholder='Select countries'
      searchLabel='Search countries'
      searchPlaceholder='Search countries...'
      loadingMessage='Loading countries...'
      emptyMessage='No countries found.'
      options={options}
      value={value}
      loading={loading}
      onValueChange={onValueChange}
    />
  )
}

function formatTriggerLabel(country: Country) {
  return `${country.name_en} (${country.code})`
}

function formatOptionTitle(country: Country) {
  return `${country.name_zh} (${country.code})`
}

function formatOptionLabel(country: Country) {
  return `${country.name_zh} ${country.name_en} ${country.code}`
}
