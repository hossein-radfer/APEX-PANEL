'use client'

import { Control } from 'react-hook-form'
import { CreatePeerRequest } from '@/schema/peers.ts'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form.tsx'
import { Input } from '@/components/ui/input.tsx'
import { PasswordInput } from '@/components/password-input.tsx'

interface Props {
  control: Control<CreatePeerRequest>
}

export function GenerateKeyField({ control }: Props) {
  return (
    <div className='space-y-4'>
      <div className='flex items-end gap-2'>
        <FormField
          control={control}
          name='private_key'
          render={({ field }) => (
            <FormItem className='flex-grow'>
              <FormLabel>کلید خصوصی</FormLabel>
              <FormControl>
                <PasswordInput
                  className='bg-input/30'
                  {...field}
                  placeholder='کلید خصوصی'
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={control}
        name='public_key'
        render={({ field }) => (
          <FormItem>
            <FormLabel>کلید عمومی</FormLabel>
            <FormControl>
              <Input {...field} placeholder='کلید عمومی' />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
