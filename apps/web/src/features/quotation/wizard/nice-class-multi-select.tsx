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
      ariaLabel='商标类别'
      placeholder='请选择商标类别'
      searchLabel='搜索商标类别'
      searchPlaceholder='按类别编号或名称搜索'
      loadingMessage='正在加载商标类别…'
      emptyMessage='未找到匹配的商标类别。'
      options={options}
      value={value}
      loading={loading}
      onValueChange={onValueChange}
    />
  )
}

function formatTriggerLabel(category: NiceCategory) {
  return `第 ${category.code} 类`
}

function formatOptionTitle(category: NiceCategory) {
  return `第 ${category.code} 类`
}

function formatOptionDescription(category: NiceCategory) {
  return category.name_zh
}

function formatOptionLabel(category: NiceCategory) {
  return `${formatOptionTitle(category)} ${category.name_zh}`
}
