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
import type { NiceCategory } from '@/features/catalog/types'

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
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const selectedCodes = useMemo(() => new Set(value), [value])
  const selectedCategories = useMemo(
    () => categories.filter((category) => selectedCodes.has(category.code)),
    [categories, selectedCodes]
  )
  const filteredCategories = useMemo(
    () => filterCategories(categories, query),
    [categories, query]
  )

  const filteredCodes = filteredCategories.map((category) => category.code)
  const allFilteredSelected =
    filteredCodes.length > 0 &&
    filteredCodes.every((code) => selectedCodes.has(code))
  const summary = selectedCategories.map(formatTriggerLabel)
  const accessibleSummary = summary.length
    ? summary.join(', ')
    : 'Select nice classes'

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setQuery('')
    }
  }

  const setChecked = (code: number, checked: boolean) => {
    if (checked) {
      onValueChange(value.includes(code) ? value : [...value, code])
      return
    }
    onValueChange(value.filter((item) => item !== code))
  }

  const toggleFiltered = () => {
    if (filteredCodes.length === 0) return

    if (allFilteredSelected) {
      onValueChange(value.filter((code) => !filteredCodes.includes(code)))
      return
    }

    const next = [...value]
    for (const code of filteredCodes) {
      if (!next.includes(code)) {
        next.push(code)
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
          aria-label={`Nice Classes: ${accessibleSummary}`}
          className='h-auto min-h-11 w-full justify-between px-3 py-2 text-left'
        >
          <span className='flex min-w-0 flex-1 flex-wrap items-center gap-1.5'>
            {selectedCategories.length === 0 ? (
              <span className='text-muted-foreground'>Select nice classes</span>
            ) : (
              <>
                {selectedCategories.slice(0, 2).map((category) => (
                  <Badge
                    key={category.code}
                    variant='secondary'
                    className='max-w-full'
                  >
                    {formatTriggerLabel(category)}
                  </Badge>
                ))}
                {selectedCategories.length > 2 && (
                  <Badge variant='outline'>
                    +{selectedCategories.length - 2}
                  </Badge>
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
            aria-label='Search nice classes'
            placeholder='Search code or names...'
            value={query}
            onValueChange={setQuery}
          />
          <div className='flex items-center justify-between gap-2 border-b p-2'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={toggleFiltered}
              disabled={filteredCategories.length === 0}
            >
              <Check data-icon='inline-start' />
              Select All
            </Button>
            <div className='flex items-center gap-1'>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => onValueChange([])}
                disabled={value.length === 0}
              >
                Clear
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => handleOpenChange(false)}
              >
                Close
              </Button>
            </div>
          </div>
          <CommandList className='max-h-none overflow-hidden'>
            {loading ? (
              <CommandEmpty>Loading nice classes...</CommandEmpty>
            ) : filteredCategories.length === 0 ? (
              <CommandEmpty>No nice classes found.</CommandEmpty>
            ) : (
              <ScrollArea className='h-72'>
                <CommandGroup>
                  {filteredCategories.map((category) => {
                    const checked = selectedCodes.has(category.code)
                    const checkboxId = `${id}-${category.code}`

                    return (
                      <label
                        key={category.code}
                        htmlFor={checkboxId}
                        className={cn(
                          'flex min-h-12 cursor-pointer items-start gap-3 rounded-sm px-2 py-2 text-sm outline-hidden transition-colors hover:bg-accent hover:text-accent-foreground',
                          checked && 'bg-accent/50'
                        )}
                      >
                        <Checkbox
                          id={checkboxId}
                          checked={checked}
                          aria-label={formatOptionLabel(category)}
                          onCheckedChange={(nextChecked) =>
                            setChecked(category.code, nextChecked === true)
                          }
                        />
                        <span className='grid min-w-0 gap-0.5 leading-tight'>
                          <span className='font-medium'>
                            {formatOptionTitle(category)}
                          </span>
                          <span className='text-xs text-muted-foreground'>
                            {formatOptionDescription(category)}
                          </span>
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

function filterCategories(categories: NiceCategory[], query: string) {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return categories

  return categories.filter((category) =>
    [
      String(category.code),
      category.name_zh,
      category.name_en,
      category.description_zh ?? '',
      category.description_en ?? '',
    ]
      .join(' ')
      .toLowerCase()
      .includes(normalizedQuery)
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
