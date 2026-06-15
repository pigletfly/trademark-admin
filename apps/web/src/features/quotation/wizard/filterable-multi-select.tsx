import { useMemo, useState } from 'react'
import { Check, ChevronsUpDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'

type FilterableMultiSelectValue = string | number

export interface FilterableMultiSelectOption<
  TValue extends FilterableMultiSelectValue,
> {
  value: TValue
  triggerLabel: string
  optionTitle: string
  optionDescription?: string
  accessibleLabel: string
  searchText: string
}

interface FilterableMultiSelectProps<
  TValue extends FilterableMultiSelectValue,
> {
  id: string
  ariaLabel: string
  placeholder: string
  searchLabel: string
  searchPlaceholder: string
  loadingMessage: string
  emptyMessage: string
  options: Array<FilterableMultiSelectOption<TValue>>
  value: TValue[]
  onValueChange: (value: TValue[]) => void
  loading?: boolean
}

export function FilterableMultiSelect<
  TValue extends FilterableMultiSelectValue,
>({
  id,
  ariaLabel,
  placeholder,
  searchLabel,
  searchPlaceholder,
  loadingMessage,
  emptyMessage,
  options,
  value,
  onValueChange,
  loading = false,
}: FilterableMultiSelectProps<TValue>) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const selectedValues = useMemo(() => new Set(value), [value])
  const selectedOptions = useMemo(
    () => options.filter((option) => selectedValues.has(option.value)),
    [options, selectedValues]
  )
  const filteredOptions = useMemo(
    () => filterOptions(options, query),
    [options, query]
  )

  const filteredValues = filteredOptions.map((option) => option.value)
  const allFilteredSelected =
    filteredValues.length > 0 &&
    filteredValues.every((optionValue) => selectedValues.has(optionValue))
  const summary = selectedOptions.map((option) => option.triggerLabel)
  const accessibleSummary = summary.length ? summary.join(', ') : placeholder

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setQuery('')
    }
  }

  const setChecked = (optionValue: TValue, checked: boolean) => {
    if (checked) {
      onValueChange(
        value.includes(optionValue) ? value : [...value, optionValue]
      )
      return
    }
    onValueChange(value.filter((item) => item !== optionValue))
  }

  const toggleFiltered = () => {
    if (filteredValues.length === 0) return

    if (allFilteredSelected) {
      onValueChange(value.filter((item) => !filteredValues.includes(item)))
      return
    }

    const next = [...value]
    for (const optionValue of filteredValues) {
      if (!next.includes(optionValue)) {
        next.push(optionValue)
      }
    }
    onValueChange(next)
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type='button'
          variant='outline'
          role='combobox'
          aria-expanded={open}
          aria-label={`${ariaLabel}: ${accessibleSummary}`}
          className='h-auto min-h-11 w-full justify-between px-3 py-2 text-left'
        >
          <span className='flex min-w-0 flex-1 flex-wrap items-center gap-1.5'>
            {selectedOptions.length === 0 ? (
              <span className='text-muted-foreground'>{placeholder}</span>
            ) : (
              <>
                {selectedOptions.slice(0, 2).map((option) => (
                  <Badge
                    key={String(option.value)}
                    variant='secondary'
                    className='max-w-full'
                  >
                    {option.triggerLabel}
                  </Badge>
                ))}
                {selectedOptions.length > 2 && (
                  <Badge variant='outline'>+{selectedOptions.length - 2}</Badge>
                )}
              </>
            )}
          </span>
          <ChevronsUpDown data-icon='inline-end' className='opacity-50' />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-[var(--radix-popover-trigger-width)] p-0'
      >
        <Command shouldFilter={false}>
          <CommandInput
            aria-label={searchLabel}
            placeholder={searchPlaceholder}
            value={query}
            onValueChange={setQuery}
          />
          <div className='flex items-center justify-between gap-2 border-b p-2'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={toggleFiltered}
              disabled={filteredOptions.length === 0}
            >
              <Check data-icon='inline-start' />
              全选
            </Button>
            <div className='flex items-center gap-1'>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => onValueChange([])}
                disabled={value.length === 0}
              >
                清空
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => handleOpenChange(false)}
              >
                关闭
              </Button>
            </div>
          </div>
          <CommandList className='max-h-none overflow-hidden'>
            {loading ? (
              <CommandEmpty>{loadingMessage}</CommandEmpty>
            ) : filteredOptions.length === 0 ? (
              <CommandEmpty>{emptyMessage}</CommandEmpty>
            ) : (
              <ScrollArea className='h-72'>
                <CommandGroup>
                  {filteredOptions.map((option) => {
                    const checked = selectedValues.has(option.value)
                    const checkboxId = `${id}-${option.value}`

                    return (
                      <label
                        key={String(option.value)}
                        htmlFor={checkboxId}
                        className={cn(
                          'flex min-h-12 cursor-pointer items-start gap-3 rounded-sm px-2 py-2 text-sm outline-hidden transition-colors hover:bg-accent hover:text-accent-foreground',
                          checked && 'bg-accent/50'
                        )}
                      >
                        <Checkbox
                          id={checkboxId}
                          checked={checked}
                          aria-label={option.accessibleLabel}
                          onCheckedChange={(nextChecked) =>
                            setChecked(option.value, nextChecked === true)
                          }
                        />
                        <span className='grid min-w-0 gap-0.5 leading-tight'>
                          <span className='font-medium'>
                            {option.optionTitle}
                          </span>
                          {option.optionDescription && (
                            <span className='text-xs text-muted-foreground'>
                              {option.optionDescription}
                            </span>
                          )}
                        </span>
                      </label>
                    )
                  })}
                </CommandGroup>
              </ScrollArea>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

function filterOptions<TValue extends FilterableMultiSelectValue>(
  options: Array<FilterableMultiSelectOption<TValue>>,
  query: string
) {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return options

  return options.filter((option) =>
    option.searchText.toLowerCase().includes(normalizedQuery)
  )
}
