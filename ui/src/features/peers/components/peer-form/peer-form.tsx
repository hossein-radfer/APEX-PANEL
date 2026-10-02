'use client'

import { CreatePeerRequest } from '@/schema/peers.ts'
import { Form } from '@/components/ui/form'
import { PeerFormFields } from '@/features/peers/components/peer-form/peer-form-fields.tsx'
import { PeerFormSkeleton } from '@/features/peers/components/peer-form/peer-form-skeleton.tsx'
import { usePeerForm } from '@/features/peers/components/peer-form/use-peer-form.tsx'

interface Props {
  currentRow?: Partial<CreatePeerRequest>
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is creating/editing this peer on behalf of a
  // specific reseller (admin "Reseller Peers" page).
  targetResellerId?: number
  formId?: string
}

export function PeerForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
  formId = 'peer-form',
}: Props) {
  const {
    form,
    onSubmit,
    isDefaultsReady,
    interfacesList,
    isInterfacesLoading,
    interfacesError,
  } = usePeerForm({
    currentRow,
    onClose,
    setIsLoading,
    targetResellerId,
  })

  if (!isDefaultsReady) {
    return <PeerFormSkeleton />
  }

  return (
    <Form {...form}>
      <form
        id={formId}
        onSubmit={form.handleSubmit(onSubmit)}
        className='space-y-4'
      >
        <PeerFormFields
          control={form.control}
          setValue={form.setValue}
          isEdit={!!currentRow}
          interfacesOverride={
            targetResellerId !== undefined
              ? {
                  data: interfacesList,
                  isLoading: isInterfacesLoading,
                  error: interfacesError as Error | null,
                }
              : undefined
          }
        />
      </form>
    </Form>
  )
}
