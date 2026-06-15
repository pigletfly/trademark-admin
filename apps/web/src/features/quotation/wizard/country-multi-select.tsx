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
  ariaLabel?: string
  placeholder?: string
}

export function CountryMultiSelect({
  id,
  countries,
  value,
  onValueChange,
  loading = false,
  ariaLabel = '国家/地区',
  placeholder = '请选择国家/地区',
}: CountryMultiSelectProps) {
  const options = useMemo(
    () =>
      countries.map(
        (country): FilterableMultiSelectOption<string> => ({
          value: country.id,
          triggerLabel: formatTriggerLabel(country),
          optionTitle: formatOptionTitle(country),
          optionDescription: undefined,
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
      ariaLabel={ariaLabel}
      placeholder={placeholder}
      searchLabel='搜索国家/地区'
      searchPlaceholder='按国家代码或名称搜索'
      loadingMessage='正在加载国家/地区…'
      emptyMessage='未找到匹配的国家/地区。'
      options={options}
      value={value}
      loading={loading}
      onValueChange={onValueChange}
    />
  )
}

function formatTriggerLabel(country: Country) {
  return `${country.name_zh}（${country.code}）`
}

function formatOptionTitle(country: Country) {
  return `${country.name_zh}（${country.code}）`
}

function formatOptionLabel(country: Country) {
  return `${country.name_zh} ${country.code}`
}
