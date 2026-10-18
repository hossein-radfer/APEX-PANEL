'use client'

import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  CreateInterfaceRequest,
  CreateInterfaceSchema,
  UpdateInterfaceRequest,
  UpdateInterfaceSchema,
} from '@/schema/interfaces.ts'
import { toast } from 'sonner'
import { useCreateInterfaceMutation } from '@/hooks/interfaces/useCreateInterfaceMutation.ts'
import { useUpdateInterfaceMutation } from '@/hooks/interfaces/useUpdateInterfaceMutation.ts'
import { useServersListQuery } from '@/hooks/servers/useServersListQuery.ts'
import { Form } from '@/components/ui/form'
import { SimpleField } from '@/features/shared-components/simple-field.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface Props {
  currentRow?: Partial<CreateInterfaceRequest>
  onClose: () => void
  setPending: (pending: boolean) => void
}

export function InterfaceForm({ currentRow, onClose, setPending }: Props) {
  const isEdit = Boolean(currentRow)
  const { mutateAsync: createInterface, isPending: isCreateInterfacePending } =
    useCreateInterfaceMutation()
  const { mutateAsync: updateInterface, isPending: isUpdateServerPending } =
    useUpdateInterfaceMutation()

  // Server picker is create-only -- an existing interface's server is
  // fixed for its lifetime (moving it would also need to move its peers/
  // IP pool, out of scope here), mirroring DNSAccountForm's own identical
  // "panel picker only on create" convention.
  const { data: serversList = [] } = useServersListQuery(!isEdit)
  const [selectedServerId, setSelectedServerId] = useState<
    number | undefined
  >(undefined)

  // Smart default: auto-select the server when exactly one exists -- same
  // convention as DNSAccountForm's own panel auto-select.
  useEffect(() => {
    if (!isEdit && selectedServerId === undefined && serversList.length === 1) {
      setSelectedServerId(serversList[0].id)
    }
  }, [isEdit, selectedServerId, serversList])

  const form = useForm<CreateInterfaceRequest>({
    resolver: zodResolver(
      (isEdit ? UpdateInterfaceSchema : CreateInterfaceSchema) as never
    ),
    defaultValues: currentRow ?? {},
  })

  useEffect(() => {
    if (currentRow) {
      form.reset(currentRow)
    }
  }, [currentRow, form])

  useEffect(() => {
    setPending?.(isEdit ? isUpdateServerPending : isCreateInterfacePending)
  }, [isCreateInterfacePending, isUpdateServerPending, isEdit, setPending])

  const onSubmit = async (
    values: CreateInterfaceRequest | UpdateInterfaceRequest
  ) => {
    try {
      setPending(true)
      if (isEdit) {
        await updateInterface(values as UpdateInterfaceRequest)
      } else {
        await createInterface({
          ...(values as CreateInterfaceRequest),
          server_id: selectedServerId,
        })
      }

      form.reset()
      toast.success(
        isEdit
          ? 'اینترفیس با موفقیت به‌روزرسانی شد.'
          : 'اینترفیس با موفقیت ایجاد شد.',
        { duration: 5000 }
      )
      onClose()
    } finally {
      setPending(false)
    }
  }

  return (
    <Form {...form}>
      <form
        id='interface-form'
        onSubmit={form.handleSubmit(onSubmit)}
        className='space-y-4'
      >
        {/* Server picker -- only shown once there's actually more than one
            server to choose between; in the single-server case it auto-
            selects silently (see the effect above) and stays invisible,
            matching this install's current, only real deployment. */}
        {!isEdit && serversList.length > 1 && (
          <div className='space-y-2'>
            <p className='text-sm font-medium'>سرور</p>
            <Select
              value={selectedServerId ? String(selectedServerId) : undefined}
              onValueChange={(value) => setSelectedServerId(Number(value))}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='یک سرور انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {serversList.map((server) => (
                  <SelectItem key={server.id} value={String(server.id)}>
                    {server.name} ({server.ip_address})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}

        {[
          {
            name: 'comment',
            label: 'توضیحات',
            placeholder: 'توضیحات (اختیاری)',
          },
          {
            name: 'name',
            label: 'نام',
            placeholder: 'نام اینترفیس',
          },
          {
            name: 'listen_port',
            label: 'پورت API',
            placeholder: 'مثلاً 13231',
          },
        ].map(({ name, label, placeholder }) => (
          <SimpleField
            key={name}
            name={name as keyof CreateInterfaceRequest}
            label={label}
            placeholder={placeholder}
            control={form.control}
          />
        ))}
      </form>
    </Form>
  )
}
