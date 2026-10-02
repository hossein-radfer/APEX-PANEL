'use client'

import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  CreatePeerRequest,
  CreatePeerSchema,
  UpdatePeerRequest,
  UpdatePeerSchema,
} from '@/schema/peers'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { fetchPeerCredentials } from '@/api/peers.ts'
import { useCreatePeerMutation } from '@/hooks/peers/useCreatePeerMutation'
import { useUpdatePeerMutation } from '@/hooks/peers/useUpdatePeerMutation'
import { useCreatePeerForResellerMutation } from '@/hooks/peers/useCreatePeerForResellerMutation.ts'
import { useUpdatePeerForResellerMutation } from '@/hooks/peers/useUpdatePeerForResellerMutation.ts'
import { usePeerDefaults } from '@/features/peers/components/peer-form/use-peer-defaults.tsx'

export function usePeerForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
}: {
  currentRow?: Partial<CreatePeerRequest>
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is creating/editing this peer on behalf of a
  // specific reseller (admin "Reseller Peers" page).
  targetResellerId?: number
}) {
  const isEdit = !!currentRow
  const isForReseller = targetResellerId !== undefined

  const form = useForm<CreatePeerRequest>({
    resolver: zodResolver(
      (isEdit ? UpdatePeerSchema : CreatePeerSchema) as never
    ),
    defaultValues: (isEdit ? currentRow : {}) as never,
  })

  const [shouldRefetch, setShouldRefetch] = useState(false)
  const [isDefaultsReady, setIsDefaultsReady] = useState(isEdit)

  useEffect(() => {
    if (!isEdit) {
      setShouldRefetch(true)
    }
  }, [isEdit])

  const {
    interfacesList,
    isInterfacesLoading,
    interfacesError,
    serversList,
    isServersLoading,
    serversError,
  } = usePeerDefaults({ isEdit, form, shouldRefetch, targetResellerId })

  const { mutateAsync: createPeer, isPending: isCreatePeerLoading } =
    useCreatePeerMutation()
  const { mutateAsync: updatePeer, isPending: isUpdatePeerLoading } =
    useUpdatePeerMutation()
  const {
    mutateAsync: createPeerForReseller,
    isPending: isCreatePeerForResellerLoading,
  } = useCreatePeerForResellerMutation()
  const {
    mutateAsync: updatePeerForReseller,
    isPending: isUpdatePeerForResellerLoading,
  } = useUpdatePeerForResellerMutation()

  const hasGeneratedKeys = useRef(false)

  useEffect(() => {
    const generateKeys = async () => {
      if (!isEdit && !hasGeneratedKeys.current) {
        hasGeneratedKeys.current = true
        const { private_key, public_key } = await fetchPeerCredentials()
        form.setValue('private_key', private_key)
        form.setValue('public_key', public_key)
        setIsDefaultsReady(true)
      }
    }

    generateKeys()
  }, [isEdit, form])

  useEffect(() => {
    if (isEdit && currentRow) {
      form.reset(currentRow)
      setIsDefaultsReady(true)
    }
  }, [isEdit, currentRow, form])

  const isPending = isForReseller
    ? isEdit
      ? isUpdatePeerForResellerLoading
      : isCreatePeerForResellerLoading
    : isEdit
      ? isUpdatePeerLoading
      : isCreatePeerLoading

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  const onSubmit = async (values: CreatePeerRequest | UpdatePeerRequest) => {
    // A confirmed, reported bug (same root cause found and fixed in
    // v2ray-form.tsx/xui-panel-form.tsx/account-form.tsx): this block
    // previously had no try/catch, so any mutation failure was an
    // unhandled rejection that crashed the whole page to the app-wide
    // "500 -- Oops! Something went wrong" error boundary instead of
    // showing a normal toast.
    try {
      if (isForReseller) {
        if (isEdit) {
          await updatePeerForReseller({
            resellerId: targetResellerId,
            peer: values as UpdatePeerRequest,
          })
          toast.success('وایرگارد با موفقیت به‌روزرسانی شد.', { duration: 5000 })
        } else {
          await createPeerForReseller({
            resellerId: targetResellerId,
            peer: values as CreatePeerRequest,
          })
          toast.success('وایرگارد با موفقیت ایجاد شد.', { duration: 5000 })
        }
      } else if (isEdit) {
        await updatePeer(values as UpdatePeerRequest)
        toast.success('وایرگارد با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        await createPeer(values as CreatePeerRequest)
        toast.success('وایرگارد با موفقیت ایجاد شد.', { duration: 5000 })
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره وایرگارد ناموفق بود. دوباره تلاش کنید.')
      )
    }
  }

  return {
    form,
    onSubmit,
    isDefaultsReady,
    interfacesList,
    isInterfacesLoading,
    interfacesError,
    serversList,
    isServersLoading,
    serversError,
  }
}
