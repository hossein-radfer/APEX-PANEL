'use client'

import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  CreateDNSPanelRequest,
  CreateDNSPanelSchema,
  DNSPanel,
  DNSTemplateOption,
  UpdateDNSPanelRequest,
  UpdateDNSPanelSchema,
} from '@/schema/dns-panel.ts'
import { PlugZapIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateDNSPanelMutation } from '@/hooks/dns-panel/useCreateDNSPanelMutation.ts'
import { useUpdateDNSPanelMutation } from '@/hooks/dns-panel/useUpdateDNSPanelMutation.ts'
import { useTestSavedDNSPanelMutation } from '@/hooks/dns-panel/useTestSavedDNSPanelMutation.ts'
import { useTestUnsavedDNSPanelMutation } from '@/hooks/dns-panel/useTestUnsavedDNSPanelMutation.ts'
import { Badge } from '@/components/ui/badge.tsx'
import { Button } from '@/components/ui/button.tsx'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form.tsx'
import { Input } from '@/components/ui/input.tsx'
import { PasswordInput } from '@/components/password-input.tsx'

interface Props {
  currentRow?: DNSPanel
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  formId?: string
}

export function DNSPanelForm({
  currentRow,
  onClose,
  setIsLoading,
  formId = 'dns-panel-form',
}: Props) {
  const isEdit = !!currentRow
  const [templates, setTemplates] = useState<DNSTemplateOption[]>([])

  const form = useForm<CreateDNSPanelRequest | UpdateDNSPanelRequest>({
    resolver: zodResolver(
      (isEdit ? UpdateDNSPanelSchema : CreateDNSPanelSchema) as never
    ),
    defaultValues: isEdit
      ? {
          id: currentRow.id,
          name: currentRow.name,
          sale_title: currentRow.sale_title,
          api_base_url: currentRow.api_base_url,
          api_key: '',
        }
      : {
          name: '',
          sale_title: '',
          api_base_url: '',
          api_key: '',
        },
  })

  const { mutateAsync: createPanel, isPending: isCreatePending } =
    useCreateDNSPanelMutation()
  const { mutateAsync: updatePanel, isPending: isUpdatePending } =
    useUpdateDNSPanelMutation()
  const { mutateAsync: testSaved, isPending: isTestSavedPending } =
    useTestSavedDNSPanelMutation()
  const { mutateAsync: testUnsaved, isPending: isTestUnsavedPending } =
    useTestUnsavedDNSPanelMutation()

  const isTesting = isTestSavedPending || isTestUnsavedPending
  const isPending = isEdit ? isUpdatePending : isCreatePending

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  const handleTestConnection = async () => {
    try {
      let result
      if (isEdit) {
        result = await testSaved(currentRow.id)
      } else {
        const values = form.getValues()
        result = await testUnsaved({
          api_base_url: values.api_base_url ?? '',
          api_key: values.api_key ?? '',
        })
      }
      setTemplates(result.templates)
      if (result.templates.length > 0) {
        toast.success(
          `اتصال با موفقیت برقرار شد -- ${result.templates.length} پلن یافت شد.`,
          { duration: 5000 }
        )
      } else {
        toast.success('اتصال با موفقیت برقرار شد، اما هیچ پلنی یافت نشد.', {
          duration: 5000,
        })
      }
    } catch {
      toast.error(
        'تست اتصال ناموفق بود. آدرس API و کلید API را بررسی کنید.',
        { duration: 5000 }
      )
    }
  }

  const onSubmit = async (
    values: CreateDNSPanelRequest | UpdateDNSPanelRequest
  ) => {
    try {
      if (isEdit) {
        const payload = { ...values } as UpdateDNSPanelRequest
        if (!payload.api_key) {
          delete payload.api_key
        }
        await updatePanel(payload)
        toast.success('پنل DNS با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        const payload = { ...values } as CreateDNSPanelRequest
        await createPanel(payload)
        toast.success('پنل DNS با موفقیت ایجاد شد.', { duration: 5000 })
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره پنل ناموفق بود. لطفاً دوباره تلاش کنید.')
      )
    }
  }

  return (
    <Form {...form}>
      <form
        id={formId}
        onSubmit={form.handleSubmit(onSubmit)}
        className='space-y-4'
      >
        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>نام</FormLabel>
                <FormControl>
                  <Input placeholder='نام داخلی پنل' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='sale_title'
            render={({ field }) => (
              <FormItem>
                <FormLabel>عنوان فروش</FormLabel>
                <FormControl>
                  <Input
                    placeholder='نام موقعیت نمایش داده‌شده به مشتری'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  به‌جای نام داخلی، به مشتریان نمایش داده می‌شود.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='api_base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>آدرس پایه API</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://dns-panel.example.com'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='md:col-span-2 flex items-end gap-3'>
            <div className='flex-1'>
              <FormField
                control={form.control}
                name='api_key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {isEdit
                        ? 'کلید API (برای حفظ مقدار فعلی خالی بگذارید)'
                        : 'کلید API'}
                    </FormLabel>
                    <FormControl>
                      <PasswordInput
                        value={field.value ?? ''}
                        onChange={field.onChange}
                        placeholder='کلید API پنل doctor-dns'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <Button
              type='button'
              variant='outline'
              onClick={handleTestConnection}
              disabled={isTesting}
              className='gap-2'
            >
              <PlugZapIcon className='h-4 w-4' />
              {isTesting ? 'در حال تست...' : 'تست اتصال'}
            </Button>
          </div>
        </div>

        {templates.length > 0 && (
          <div className='space-y-2 rounded-lg border p-3'>
            <p className='text-sm font-medium'>پلن‌های یافت‌شده روی این پنل</p>
            <p className='text-muted-foreground text-xs'>
              این فهرست فقط جهت اطلاع است -- انتخاب پلن هنگام ساخت حساب DNS
              انجام می‌شود، نه اینجا.
            </p>
            <div className='flex flex-wrap gap-2'>
              {templates.map((template) => (
                <Badge key={template.id} variant='secondary'>
                  {template.name}
                  {template.is_default ? ' (پیش‌فرض)' : ''}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </form>
    </Form>
  )
}
