'use client'

import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  CreateXuiPanelRequest,
  CreateXuiPanelSchema,
  UpdateXuiPanelRequest,
  UpdateXuiPanelSchema,
  XuiPanel,
  XuiPanelInbound,
} from '@/schema/xui-panel.ts'
import { fetchServersList } from '@/api/servers.ts'
import { PlugZapIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateXuiPanelMutation } from '@/hooks/xui-panel/useCreateXuiPanelMutation.ts'
import { useUpdateXuiPanelMutation } from '@/hooks/xui-panel/useUpdateXuiPanelMutation.ts'
import { useTestSavedXuiPanelMutation } from '@/hooks/xui-panel/useTestSavedXuiPanelMutation.ts'
import { useTestUnsavedXuiPanelMutation } from '@/hooks/xui-panel/useTestUnsavedXuiPanelMutation.ts'
import { Button } from '@/components/ui/button.tsx'
import { Separator } from '@/components/ui/separator.tsx'
import { Switch } from '@/components/ui/switch.tsx'
import { Label } from '@/components/ui/label.tsx'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'

interface Props {
  currentRow?: XuiPanel
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  formId?: string
}

export function XuiPanelForm({
  currentRow,
  onClose,
  setIsLoading,
  formId = 'xui-panel-form',
}: Props) {
  const isEdit = !!currentRow
  const [inbounds, setInbounds] = useState<XuiPanelInbound[]>([])
  // Container mapping (item 9) is shown behind its own toggle rather than
  // always-visible fields -- most panels are NOT hosted inside a RouterOS
  // container, so defaulting the section open would make the common case
  // look like it's missing required configuration.
  const [containerMappingEnabled, setContainerMappingEnabled] = useState(
    isEdit && currentRow.container_server_id != null
  )

  const { data: servers = [] } = useQuery({
    queryKey: ['servers-list'],
    queryFn: fetchServersList,
    enabled: containerMappingEnabled,
  })

  const form = useForm<CreateXuiPanelRequest | UpdateXuiPanelRequest>({
    resolver: zodResolver(
      (isEdit ? UpdateXuiPanelSchema : CreateXuiPanelSchema) as never
    ),
    defaultValues: isEdit
      ? {
          id: currentRow.id,
          name: currentRow.name,
          sale_title: currentRow.sale_title,
          api_base_url: currentRow.api_base_url,
          username: currentRow.username,
          password: '',
          default_inbound_id: currentRow.default_inbound_id,
          protocol: currentRow.protocol,
          sub_base_url: currentRow.sub_base_url,
          container_server_id: currentRow.container_server_id ?? undefined,
          container_name: currentRow.container_name ?? undefined,
        }
      : {
          name: '',
          sale_title: '',
          api_base_url: '',
          username: '',
          password: '',
          default_inbound_id: 0,
          protocol: '',
          sub_base_url: '',
        },
  })

  const { mutateAsync: createPanel, isPending: isCreatePending } =
    useCreateXuiPanelMutation()
  const { mutateAsync: updatePanel, isPending: isUpdatePending } =
    useUpdateXuiPanelMutation()
  const { mutateAsync: testSaved, isPending: isTestSavedPending } =
    useTestSavedXuiPanelMutation()
  const { mutateAsync: testUnsaved, isPending: isTestUnsavedPending } =
    useTestUnsavedXuiPanelMutation()

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
          username: values.username ?? '',
          password: values.password ?? '',
        })
      }
      setInbounds(result.inbounds)
      if (result.inbounds.length > 0) {
        toast.success(
          `اتصال با موفقیت برقرار شد -- ${result.inbounds.length} inbound یافت شد.`,
          { duration: 5000 }
        )
      } else {
        toast.success('اتصال با موفقیت برقرار شد، اما هیچ inbound یافت نشد.', {
          duration: 5000,
        })
      }
    } catch {
      toast.error(
        'تست اتصال ناموفق بود. آدرس API و اطلاعات ورود را بررسی کنید.',
        { duration: 5000 }
      )
    }
  }

  const onSubmit = async (
    values: CreateXuiPanelRequest | UpdateXuiPanelRequest
  ) => {
    // A confirmed, reported bug (same root cause found and fixed in
    // v2ray-form.tsx): this block previously had no try/catch, so any
    // mutation failure was an unhandled rejection that crashed the whole
    // page to the app-wide "500 -- Oops! Something went wrong" error
    // boundary instead of showing a normal toast.
    try {
      if (isEdit) {
        const payload = { ...values } as UpdateXuiPanelRequest
        if (!payload.password) {
          delete payload.password
        }
        if (!containerMappingEnabled) {
          // The admin turned the mapping section off -- explicitly clear
          // it rather than just omitting the fields, since an omitted
          // field on UpdateXuiPanelRequest means "leave unchanged," which
          // would silently keep a stale mapping around.
          delete payload.container_server_id
          delete payload.container_name
          payload.clear_container_mapping = true
        }
        await updatePanel(payload)
        toast.success('پنل X-UI با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        const payload = { ...values } as CreateXuiPanelRequest
        if (!containerMappingEnabled) {
          delete payload.container_server_id
          delete payload.container_name
        }
        await createPanel(payload)
        toast.success('پنل X-UI با موفقیت ایجاد شد.', { duration: 5000 })
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
                      placeholder='https://panel.example.com:54321'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='username'
            render={({ field }) => (
              <FormItem>
                <FormLabel>نام کاربری</FormLabel>
                <FormControl>
                  <Input placeholder='نام کاربری x-ui' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='password'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {isEdit ? 'رمز عبور (برای حفظ مقدار فعلی خالی بگذارید)' : 'رمز عبور'}
                </FormLabel>
                <FormControl>
                  <PasswordInput
                    value={field.value ?? ''}
                    onChange={field.onChange}
                    placeholder='رمز عبور x-ui'
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='protocol'
            render={({ field }) => (
              <FormItem>
                <FormLabel>پروتکل</FormLabel>
                <FormControl>
                  <Input placeholder='vless / vmess / trojan' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='sub_base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>آدرس پایه اشتراک</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://panel.example.com:2096/sub'
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
                name='default_inbound_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Inbound پیش‌فرض</FormLabel>
                    <Select
                      onValueChange={(value) => field.onChange(Number(value))}
                      value={field.value ? String(field.value) : undefined}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue
                            placeholder={
                              inbounds.length === 0
                                ? 'برای بارگذاری inboundها، اتصال را تست کنید'
                                : 'یک inbound انتخاب کنید'
                            }
                          />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {inbounds.map((inbound) => (
                          <SelectItem
                            key={inbound.id}
                            value={String(inbound.id)}
                          >
                            {inbound.remark} ({inbound.protocol}) #{inbound.id}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
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

        <Separator />

        <div className='space-y-3'>
          <div className='flex items-center justify-between rounded-lg border p-3'>
            <div>
              <Label htmlFor='container-mapping-enabled' className='text-sm font-medium'>
                میزبانی‌شده درون یک کانتینر RouterOS
              </Label>
              <p className='text-muted-foreground text-xs'>
                اگر این پنل x-ui درون یک کانتینر Mikrotik RouterOS اجرا
                می‌شود، آن را اینجا نگاشت کنید تا قطعی پنل به‌جای یک خطای
                اتصال عمومی، به‌صورت «کانتینر متوقف شده» تشخیص داده شود.
              </p>
            </div>
            <Switch
              id='container-mapping-enabled'
              checked={containerMappingEnabled}
              onCheckedChange={setContainerMappingEnabled}
            />
          </div>

          {containerMappingEnabled && (
            <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='container_server_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>سرور Mikrotik</FormLabel>
                    <Select
                      onValueChange={(value) => field.onChange(Number(value))}
                      value={field.value ? String(field.value) : undefined}
                    >
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue placeholder='روتر میزبان این کانتینر را انتخاب کنید' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {servers.map((server) => (
                          <SelectItem key={server.id} value={String(server.id)}>
                            {server.name} ({server.ip_address})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='container_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>نام کانتینر</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='مثلاً xui-panel-1'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      مقدار فیلد Name کانتینر همان‌طور که روی روتر نمایش
                      داده می‌شود (لیست RouterOS /container).
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </div>
      </form>
    </Form>
  )
}
