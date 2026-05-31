import { useMemo } from 'react'
import type { NiceCategory } from '@/features/catalog/types'
import {
  FilterableMultiSelect,
  type FilterableMultiSelectOption,
} from './filterable-multi-select'

interface NiceClassMultiSelectProps {
  id: string
  categories: NiceCategory[]
  value: number[]
  onValueChange: (value: number[]) => void
  loading?: boolean
}

export function NiceClassMultiSelect({
  id,
  categories,
  value,
  onValueChange,
  loading = false,
}: NiceClassMultiSelectProps) {
  const options = useMemo(
    () =>
      categories.map(
        (category): FilterableMultiSelectOption<number> => ({
          value: category.code,
          triggerLabel: formatTriggerLabel(category),
          optionTitle: formatOptionTitle(category),
          optionDescription: formatOptionDescription(category),
          accessibleLabel: formatOptionLabel(category),
          searchText: [
            String(category.code),
            category.name_zh,
            category.name_en,
            category.description_zh ?? '',
            category.description_en ?? '',
          ].join(' '),
        })
      ),
    [categories]
  )

  return (
    <FilterableMultiSelect
      id={id}
      ariaLabel='Nice Classes'
      placeholder='Select nice classes'
      searchLabel='Search nice classes'
      searchPlaceholder='Search code or names...'
      loadingMessage='Loading nice classes...'
      emptyMessage='No nice classes found.'
      options={options}
      value={value}
      loading={loading}
      onValueChange={onValueChange}
    />
  )
}

function formatTriggerLabel(category: NiceCategory) {
  return `Class ${category.code}`
}

function formatOptionTitle(category: NiceCategory) {
  return `Class ${category.code}`
}

function formatOptionDescription(category: NiceCategory) {
  return `${category.name_zh} / ${category.name_en}`
}

function formatOptionLabel(category: NiceCategory) {
  return `${formatOptionTitle(category)} ${category.name_en}`
}
