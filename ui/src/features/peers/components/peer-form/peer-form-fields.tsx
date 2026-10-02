'use client'

import { Control, UseFormSetValue } from 'react-hook-form'
import { CreatePeerRequest } from '@/schema/peers'
import { Interface } from '@/schema/interfaces.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery'
import { BandwidthField } from '@/features/peers/components/fields/bandwidth-field'
import { ExpireDateField } from '@/features/peers/components/fields/expire-date-field'
import { GenerateKeyField } from '@/features/peers/components/fields/generate-key-field'
import { InterfaceSelect } from '@/features/peers/components/fields/interface-select'
import { TrafficInput } from '@/features/peers/components/fields/taffic-limit-field'
import { SimpleField } from '@/features/shared-components/simple-field'

interface Props {
  control: Control<CreatePeerRequest>
  setValue: UseFormSetValue<CreatePeerRequest>
  isEdit: boolean
  // When set (the admin "Reseller Peers" page), the interface list is
  // scoped to the target reseller and fetched by the caller instead of
  // this component's own global useInterfacesListQuery below.
  interfacesOverride?: {
    data: Interface[]
    isLoading?: boolean
    error?: unknown
  }
}

export function PeerFormFields({
  control,
  setValue,
  isEdit,
  interfacesOverride,
}: Props) {
  const {
    data: fetchedInterfacesList = [],
    isLoading: isFetchedInterfacesLoading,
    error: fetchedInterfacesError,
  } = useInterfacesListQuery()

  const interfacesList = interfacesOverride?.data ?? fetchedInterfacesList
  const isInterfacesLoading =
    interfacesOverride?.isLoading ?? isFetchedInterfacesLoading
  const interfacesError = interfacesOverride?.error ?? fetchedInterfacesError

  return (
    <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
      <SimpleField
        name='name'
        label='نام'
        placeholder='نام وایرگارد'
        control={control}
      />
      <SimpleField
        name='telegram_username'
        label='نام کاربری تلگرام'
        placeholder='مثلاً @username (اختیاری)'
        control={control}
      />
      <div className='md:col-span-2'>
        <SimpleField
          name='comment'
          label='توضیحات'
          placeholder='توضیحات (اختیاری)'
          control={control}
        />
      </div>

      {!isEdit && (
        <div className='md:col-span-2'>
          <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
            <InterfaceSelect
              name='interface_id'
              label='اینترفیس'
              control={control}
              setValue={setValue}
              options={interfacesList}
              isLoading={isInterfacesLoading}
              error={interfacesError}
            />
            <SimpleField
              name='endpoint'
              label='Endpoint'
              placeholder='مثلاً 185.51.200.10'
              control={control}
            />
            <div className='md:col-span-2'>
              <GenerateKeyField control={control} />
            </div>
          </div>
        </div>
      )}

      <SimpleField
        name='allowed_address'
        label='آدرس مجاز'
        placeholder='مثلاً 10.0.0.2/32'
        control={control}
      />
      <SimpleField
        name='dns_servers'
        label='سرورهای DNS'
        placeholder='مثلاً 1.1.1.1, 8.8.8.8 (اختیاری)'
        control={control}
      />
      <SimpleField
        name='persistent_keepalive'
        label='Persistent Keepalive'
        placeholder='مثلاً 00:00:25 (اختیاری)'
        control={control}
      />

      <TrafficInput control={control} />
      <ExpireDateField control={control} />

      <div className='space-y-4 md:col-span-2'>
        <BandwidthField
          name='download_bandwidth'
          label='پهنای باند دانلود'
          control={control}
        />
        <BandwidthField
          name='upload_bandwidth'
          label='پهنای باند آپلود'
          control={control}
        />
      </div>
    </div>
  )
}
