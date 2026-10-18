'use client'

import { Control } from 'react-hook-form'
import { Interface } from '@/schema/interfaces.ts'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { serverBadgeColorClass } from '@/lib/server-badge-colors.ts'

interface Props {
  name: 'interface_id'
  label: string
  control: Control<any>
  setValue: (name: 'interface_id', value: number) => void
  options: Array<Interface>
  isLoading?: boolean
  error?: unknown
}

export function InterfaceSelect({
  name,
  label,
  control,
  options,
  isLoading,
  error,
}: Props) {
  // Only show the per-server badge/color once there's actually more than
  // one server among the candidates -- in the common single-server case
  // (today's only real deployment) every interface already implicitly
  // belongs to the same server, so the dot would be pure visual noise.
  const distinctServerCount = new Set(options.map((opt) => opt.server_id)).size
  const showServerBadge = distinctServerCount > 1

  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Select
              onValueChange={(selectedId) => {
                const selected = options.find(
                  (opt) => opt.id.toString() === selectedId
                )
                if (selected) {
                  field.onChange(selected.id) // sets form field interface_id = selected.id
                }
              }}
              value={field.value?.toString()}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='انتخاب اینترفیس' />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectLabel>اینترفیس‌ها</SelectLabel>
                  {isLoading ? (
                    <SelectItem value='loading' disabled>
                      در حال بارگذاری...
                    </SelectItem>
                  ) : error ? (
                    <SelectItem value='error' disabled>
                      خطا در بارگذاری اینترفیس‌ها
                    </SelectItem>
                  ) : options.length === 0 ? (
                    <SelectItem value='empty' disabled>
                      اینترفیسی موجود نیست
                    </SelectItem>
                  ) : (
                    options.map((item) => (
                      <SelectItem key={item.id} value={item.id.toString()}>
                        {showServerBadge ? (
                          <span className='inline-flex items-center gap-1.5'>
                            <span
                              className={`inline-block size-2 shrink-0 rounded-full ${serverBadgeColorClass(item.server_id)}`}
                              aria-hidden='true'
                            />
                            {item.name}
                            <span className='text-muted-foreground text-xs'>
                              ({item.server_name})
                            </span>
                          </span>
                        ) : (
                          item.name
                        )}
                      </SelectItem>
                    ))
                  )}
                </SelectGroup>
              </SelectContent>
            </Select>
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
