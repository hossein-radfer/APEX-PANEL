'use client'

import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import {
  CreateDNSAccountRequest,
  CreateDNSAccountSchema,
  DNSAccount,
  DNSAccountStatusEnum,
  UpdateDNSAccountRequest,
  UpdateDNSAccountSchema,
} from '@/schema/dns-account.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { DNSPanel } from '@/schema/dns-panel.ts'
import { useDNSPlansListQuery } from '@/hooks/dns-plan/useDNSPlansListQuery.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateDNSAccountMutation } from '@/hooks/dns-account/useCreateDNSAccountMutation.ts'
import { useUpdateDNSAccountMutation } from '@/hooks/dns-account/useUpdateDNSAccountMutation.ts'
import { useCreateDNSAccountForResellerMutation } from '@/hooks/dns-account/useCreateDNSAccountForResellerMutation.ts'
import { useUpdateDNSAccountForResellerMutation } from '@/hooks/dns-account/useUpdateDNSAccountForResellerMutation.ts'
import { useDNSPanelsListQuery } from '@/hooks/dns-panel/useDNSPanelsListQuery.ts'
import { useAssignedDNSPanelSummariesQuery } from '@/hooks/dns-account/useAssignedDNSPanelSummariesQuery.ts'
import { useTestSavedDNSPanelMutation } from '@/hooks/dns-panel/useTestSavedDNSPanelMutation.ts'
import { useTestSavedDNSPanelForResellerMutation } from '@/hooks/dns-panel/useTestSavedDNSPanelForResellerMutation.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { COMMON_COUNTRIES } from '@/features/dns-accounts/lib/countries.ts'
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
import { Checkbox } from '@/components/ui/checkbox.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'

// Module-level stable empty array -- see EMPTY_PANELS's own doc comment in
// v2ray-packages/components/v2ray-form.tsx for the confirmed, reported
// infinite-render-loop bug this convention avoids (an inline `= []`
// destructure default creates a brand new reference on every render while
// the query is loading).
const EMPTY_PANELS: DNSPanel[] = []

// PanelPickerEntry can come from either the admin-only full-panel list
// (DNSPanel, with credentials) or the reseller-safe assigned-summary
// endpoint (DNSPanelSummary, id+name only) -- the select JSX only ever
// reads .id/.name, so this minimal shared shape covers both.
type PanelPickerEntry = { id: number; name: string }
const EMPTY_PANEL_SUMMARIES: PanelPickerEntry[] = []

interface FormValues {
  id?: number
  customer_label: string
  comment: string
  total_volume_gb: number | string
  speed_kbps: number | string
  duration_days: number | string
  max_concurrent_ips: number | string
  daily_ip_registration_limit: number | string
  status?: 'active' | 'suspended' | 'expired'
}

interface Props {
  currentRow?: DNSAccount
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is creating/editing this account on behalf of a
  // specific reseller (admin "Resellers DNS" page).
  targetResellerId?: number
  formId?: string
}

export function DNSAccountForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
  formId = 'dns-account-form',
}: Props) {
  const isEdit = !!currentRow
  const isForReseller = targetResellerId !== undefined

  const form = useForm<FormValues>({
    defaultValues: isEdit
      ? {
          id: currentRow.id,
          customer_label: currentRow.customer_label ?? '',
          comment: currentRow.comment ?? '',
          total_volume_gb:
            currentRow.total_volume_bytes > 0
              ? Number((currentRow.total_volume_bytes / BYTES_PER_GB).toFixed(2))
              : 0,
          speed_kbps: currentRow.speed_kbps,
          duration_days: currentRow.duration_days,
          max_concurrent_ips: currentRow.max_concurrent_ips,
          daily_ip_registration_limit: currentRow.daily_ip_registration_limit,
          status: currentRow.status,
        }
      : {
          customer_label: '',
          comment: '',
          total_volume_gb: '',
          speed_kbps: '',
          duration_days: '',
          max_concurrent_ips: 1,
          daily_ip_registration_limit: '',
        },
  })

  const { mutateAsync: createAccount, isPending: isCreatePending } =
    useCreateDNSAccountMutation()
  const { mutateAsync: updateAccount, isPending: isUpdatePending } =
    useUpdateDNSAccountMutation()
  const {
    mutateAsync: createAccountForReseller,
    isPending: isCreateForResellerPending,
  } = useCreateDNSAccountForResellerMutation()
  const {
    mutateAsync: updateAccountForReseller,
    isPending: isUpdateForResellerPending,
  } = useUpdateDNSAccountForResellerMutation()
  const { mutateAsync: testSavedPanel, isPending: isTestPending } =
    useTestSavedDNSPanelMutation()
  const {
    mutateAsync: testSavedPanelForReseller,
    isPending: isTestForResellerPending,
  } = useTestSavedDNSPanelForResellerMutation()

  const isPending = isForReseller
    ? isEdit
      ? isUpdateForResellerPending
      : isCreateForResellerPending
    : isEdit
      ? isUpdatePending
      : isCreatePending

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  // Panel selection only matters on create -- an existing account's panel
  // is fixed for its lifetime (one DNS account lives on exactly one DNS
  // panel, unlike V2Ray's multi-panel fan-out), so edit never shows this
  // picker at all.
  const authRole = useAuthStore((state) => state.auth.admin?.role)
  const authResellerId = useAuthStore((state) => state.auth.admin?.reseller_id)

  const scopedResellerId =
    targetResellerId ?? (authRole === 'reseller' ? (authResellerId ?? undefined) : undefined)

  // GET /api/dns-panel (the full panel list, with connection credentials)
  // is admin-only server-side -- only fetch it when this form is actually
  // being used in an admin-direct context (scopedResellerId === undefined).
  // See useDNSPanelsListQuery's own doc comment / useXuiPanelsListQuery's
  // identical precedent for the confirmed 403-redirect bug this avoids.
  const { data: allPanels = EMPTY_PANELS } = useDNSPanelsListQuery(
    scopedResellerId === undefined
  )
  const { data: assignedPanelSummaries } =
    useAssignedDNSPanelSummariesQuery(scopedResellerId)

  const candidatePanels: PanelPickerEntry[] = useMemo(() => {
    if (scopedResellerId === undefined) {
      return allPanels
    }
    return assignedPanelSummaries ?? EMPTY_PANEL_SUMMARIES
  }, [allPanels, assignedPanelSummaries, scopedResellerId])

  const [selectedPanelId, setSelectedPanelId] = useState<number | undefined>(
    isEdit ? currentRow.panel_id : undefined
  )

  // Smart default on CREATE: auto-select the panel when exactly one
  // candidate exists. On EDIT, the panel is fixed to the account's own
  // current panel_id (set in useState's initializer above) and never
  // re-seeded from candidatePanels.
  useEffect(() => {
    if (isEdit) return
    if (selectedPanelId === undefined && candidatePanels.length === 1) {
      setSelectedPanelId(candidatePanels[0].id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isEdit, candidatePanels])

  const [templateId, setTemplateId] = useState<number | undefined>(
    currentRow?.template_id ?? undefined
  )
  const [templateOptions, setTemplateOptions] = useState<
    { id: number; name: string; is_default: boolean }[]
  >([])

  // Loads the selectable plan/template list for the chosen panel -- only
  // meaningful once a panel is picked, since templates live on the panel
  // itself (DNSTemplateOption comes back from the panel's own test-
  // connection call). Edit mode doesn't re-load this: the account's
  // existing template_id is kept as-is unless the admin explicitly wants
  // to change it, and re-testing an edited account's OWN panel is already
  // possible from the DNS Panels page.
  const handleLoadTemplates = async () => {
    if (!isEdit && selectedPanelId === undefined) {
      toast.error('ابتدا یک پنل DNS انتخاب کنید.', { duration: 5000 })
      return
    }
    const panelId = isEdit ? currentRow.panel_id : (selectedPanelId as number)
    try {
      // GET/POST /dns-panel/* (testSavedPanel) is admin-only server-side --
      // a confirmed, reported bug: this button previously called it
      // unconditionally, so a reseller creating/editing their own DNS
      // account always got a 403 here. scopedResellerId (already computed
      // above for the panel-list query) tells us whether the CALLER is a
      // reseller, so route to the reseller-scoped test endpoint instead in
      // that case -- mirrors every other admin/reseller dual-path call in
      // this form.
      const result =
        scopedResellerId === undefined
          ? await testSavedPanel(panelId)
          : await testSavedPanelForReseller({
              resellerId: scopedResellerId,
              id: panelId,
            })
      setTemplateOptions(result.templates)
      if (result.templates.length === 0) {
        toast.success('اتصال برقرار شد، اما هیچ پلنی روی این پنل یافت نشد.', {
          duration: 5000,
        })
      }
    } catch {
      toast.error('دریافت فهرست پلن‌ها از این پنل ناموفق بود.', {
        duration: 5000,
      })
    }
  }

  // Local Apex-side pricing plan (bronze/silver/gold tiers) -- distinct
  // from templateId/templateOptions above, which is the REMOTE doctor-dns
  // plan living on the panel itself. Picking one here just pre-fills the
  // limit fields below from the plan's bundle; the admin can still edit
  // any of them by hand afterward (explicit values always win server-side).
  // GET /api/dns-plan is admin-only server-side (same as GET /api/dns-panel
  // above) -- a confirmed, reported bug: this previously fetched
  // unconditionally, so a RESELLER opening this form (to create their own
  // DNS account) got an instant 403 the moment the form mounted, since
  // their JWT has role=reseller. Gated on the exact same scopedResellerId
  // check useDNSPanelsListQuery already uses just above.
  const { data: dnsPlans } = useDNSPlansListQuery(scopedResellerId === undefined)
  const activeDnsPlans = useMemo(
    () => (dnsPlans ?? []).filter((plan) => plan.is_active),
    [dnsPlans]
  )
  const [selectedPlanId, setSelectedPlanId] = useState<number | undefined>(
    currentRow?.plan_id ?? undefined
  )

  const applyPlanToForm = (planId: number) => {
    const plan = (dnsPlans ?? []).find((candidate) => candidate.id === planId)
    if (!plan) return
    form.setValue(
      'total_volume_gb',
      plan.total_volume_bytes > 0
        ? Number((plan.total_volume_bytes / BYTES_PER_GB).toFixed(2))
        : 0
    )
    form.setValue('speed_kbps', plan.speed_kbps)
    form.setValue('duration_days', plan.duration_days)
    form.setValue('max_concurrent_ips', plan.max_concurrent_ips)
    form.setValue(
      'daily_ip_registration_limit',
      plan.daily_ip_registration_limit
    )
  }

  const [allowedCountries, setAllowedCountries] = useState<string[]>(
    currentRow?.allowed_countries ?? []
  )

  const toggleCountry = (country: string, checked: boolean) => {
    setAllowedCountries((prev) =>
      checked ? [...prev, country] : prev.filter((existing) => existing !== country)
    )
  }

  const onSubmit = async (values: FormValues) => {
    const totalVolumeGb = values.total_volume_gb === '' ? 0 : Number(values.total_volume_gb)
    const totalVolumeBytes = Math.round(totalVolumeGb * BYTES_PER_GB)
    const speedKbps = values.speed_kbps === '' ? 0 : Number(values.speed_kbps)
    const durationDays = values.duration_days === '' ? 0 : Number(values.duration_days)
    const maxConcurrentIps = Number(values.max_concurrent_ips)
    const dailyLimit =
      values.daily_ip_registration_limit === ''
        ? 0
        : Number(values.daily_ip_registration_limit)

    if (!Number.isFinite(totalVolumeBytes) || totalVolumeBytes < 0) {
      toast.error('حجم کل باید عددی معتبر بر حسب گیگابایت باشد (۰ = نامحدود).', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(speedKbps) || speedKbps < 0) {
      toast.error('سرعت باید عددی معتبر بر حسب کیلوبیت بر ثانیه باشد (۰ = نامحدود).', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(durationDays) || durationDays < 0) {
      toast.error('مدت زمان باید عددی معتبر بر حسب روز باشد (۰ = هرگز منقضی نمی‌شود).', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(maxConcurrentIps) || maxConcurrentIps < 1) {
      toast.error('حداکثر IP هم‌زمان باید حداقل ۱ باشد.', { duration: 5000 })
      return
    }
    if (!Number.isFinite(dailyLimit) || dailyLimit < 0) {
      toast.error('سقف ثبت روزانه IP باید عددی معتبر باشد (۰ = نامحدود).', {
        duration: 5000,
      })
      return
    }
    if (!isEdit && selectedPanelId === undefined) {
      toast.error('یک پنل DNS برای این حساب انتخاب کنید.', { duration: 5000 })
      return
    }

    try {
      if (isForReseller) {
        if (isEdit) {
          const payload: UpdateDNSAccountRequest = UpdateDNSAccountSchema.parse({
            id: currentRow.id,
            customer_label: values.customer_label || null,
            comment: values.comment || null,
            total_volume_bytes: totalVolumeBytes,
            speed_kbps: speedKbps,
            duration_days: durationDays,
            template_id: templateId ?? null,
            status: values.status,
            max_concurrent_ips: maxConcurrentIps,
            daily_ip_registration_limit: dailyLimit,
            allowed_countries: allowedCountries,
            plan_id: selectedPlanId ?? null,
          })
          await updateAccountForReseller({
            resellerId: targetResellerId,
            account: payload,
          })
          toast.success('حساب با موفقیت به‌روزرسانی شد.', { duration: 5000 })
        } else {
          const payload: CreateDNSAccountRequest = CreateDNSAccountSchema.parse({
            customer_label: values.customer_label || null,
            comment: values.comment || null,
            panel_id: selectedPanelId,
            total_volume_bytes: totalVolumeBytes,
            speed_kbps: speedKbps,
            duration_days: durationDays,
            template_id: templateId ?? null,
            max_concurrent_ips: maxConcurrentIps,
            daily_ip_registration_limit: dailyLimit,
            allowed_countries: allowedCountries,
            plan_id: selectedPlanId ?? null,
          })
          await createAccountForReseller({
            resellerId: targetResellerId,
            account: payload,
          })
          toast.success('حساب با موفقیت ایجاد شد.', { duration: 5000 })
        }
      } else if (isEdit) {
        const payload: UpdateDNSAccountRequest = UpdateDNSAccountSchema.parse({
          id: currentRow.id,
          customer_label: values.customer_label || null,
          comment: values.comment || null,
          total_volume_bytes: totalVolumeBytes,
          speed_kbps: speedKbps,
          duration_days: durationDays,
          template_id: templateId ?? null,
          status: values.status,
          max_concurrent_ips: maxConcurrentIps,
          daily_ip_registration_limit: dailyLimit,
          allowed_countries: allowedCountries,
          plan_id: selectedPlanId ?? null,
        })
        await updateAccount(payload)
        toast.success('حساب با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        const payload: CreateDNSAccountRequest = CreateDNSAccountSchema.parse({
          customer_label: values.customer_label || null,
          comment: values.comment || null,
          panel_id: selectedPanelId,
          total_volume_bytes: totalVolumeBytes,
          speed_kbps: speedKbps,
          duration_days: durationDays,
          template_id: templateId ?? null,
          max_concurrent_ips: maxConcurrentIps,
          daily_ip_registration_limit: dailyLimit,
          allowed_countries: allowedCountries,
          plan_id: selectedPlanId ?? null,
        })
        await createAccount(payload)
        toast.success('حساب با موفقیت ایجاد شد.', { duration: 5000 })
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره‌ی حساب ناموفق بود. دوباره تلاش کنید.')
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
          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='customer_label'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>برچسب مشتری</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='برچسب (اختیاری)'
                      value={field.value ?? ''}
                      onChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='comment'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>یادداشت</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='یادداشت داخلی (اختیاری، فقط برای ادمین)'
                      value={field.value ?? ''}
                      onChange={field.onChange}
                    />
                  </FormControl>
                  <FormDescription>
                    فقط برای شما قابل مشاهده است -- هرگز به مشتری نمایش داده
                    نمی‌شود.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='total_volume_gb'
            render={({ field }) => (
              <FormItem>
                <FormLabel>حجم کل (گیگابایت)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='0.01'
                    min='0'
                    placeholder='۰ = نامحدود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>۰ یعنی نامحدود.</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='speed_kbps'
            render={({ field }) => (
              <FormItem>
                <FormLabel>سرعت (کیلوبیت بر ثانیه)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='0'
                    placeholder='۰ = نامحدود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>۰ یعنی نامحدود.</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='duration_days'
            render={({ field }) => (
              <FormItem>
                <FormLabel>مدت زمان (روز)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='0'
                    placeholder='۰ = هرگز منقضی نمی‌شود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>۰ یعنی هرگز منقضی نمی‌شود.</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='max_concurrent_ips'
            render={({ field }) => (
              <FormItem>
                <FormLabel>حداکثر IP هم‌زمان</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    placeholder='مثلاً 1'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  حداکثر تعداد IPهایی که این حساب می‌تواند هم‌زمان ثبت کند.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='daily_ip_registration_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>سقف ثبت روزانه IP</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='0'
                    placeholder='۰ = نامحدود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  حداکثر تعداد دفعاتی که مشتری در روز می‌تواند IP جدید ثبت
                  کند. ۰ یعنی نامحدود.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {isEdit && (
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>وضعیت</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue placeholder='یک وضعیت انتخاب کنید' />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {DNSAccountStatusEnum.options.map((status) => (
                        <SelectItem key={status} value={status}>
                          {status}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}
        </div>

        {/* Single-select DNS panel picker -- unlike V2Ray's multi-panel
            fan-out, one DNS account lives on exactly one DNS panel, fixed
            for the account's lifetime. Not shown on edit: an existing
            account's panel never changes. */}
        {!isEdit && (
          <div className='space-y-2 rounded-lg border p-4'>
            <div>
              <p className='text-sm font-medium'>پنل DNS</p>
              <p className='text-muted-foreground text-sm'>
                این حساب روی پنل DNS انتخاب‌شده در زیر ایجاد خواهد شد.
              </p>
            </div>
            {candidatePanels.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {scopedResellerId !== undefined
                  ? 'هنوز هیچ پنل DNS به این نماینده اختصاص داده نشده است -- با ادمین خود تماس بگیرید.'
                  : 'هنوز هیچ پنل DNS ثبت نشده است. ابتدا یکی را در بخش پنل‌های DNS اضافه کنید.'}
              </p>
            ) : (
              <Select
                value={selectedPanelId ? String(selectedPanelId) : undefined}
                onValueChange={(value) => {
                  setSelectedPanelId(Number(value))
                  setTemplateOptions([])
                  setTemplateId(undefined)
                }}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder='یک پنل DNS انتخاب کنید' />
                </SelectTrigger>
                <SelectContent>
                  {candidatePanels.map((panel) => (
                    <SelectItem key={panel.id} value={String(panel.id)}>
                      {panel.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        )}

        {/* Local Apex-side pricing plan (bronze/silver/gold) -- deliberately
            a separate box from the remote doctor-dns "پلن" box below it, so
            the admin never confuses the two: this one just pre-fills the
            limit fields above and can be changed/cleared freely at any
            time, unlike the panel picker above it. */}
        <div className='space-y-2 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>پلن قیمت‌گذاری</p>
            <p className='text-muted-foreground text-sm'>
              با انتخاب یک پلن، فیلدهای حجم/سرعت/مدت/محدودیت IP بالا از روی
              آن پر می‌شوند -- همچنان می‌توانید هرکدام را دستی تغییر دهید.
            </p>
          </div>
          {activeDnsPlans.length === 0 ? (
            <p className='text-muted-foreground text-sm'>
              هنوز هیچ پلن قیمت‌گذاری فعالی ثبت نشده است.
            </p>
          ) : (
            <Select
              value={selectedPlanId ? String(selectedPlanId) : undefined}
              onValueChange={(value) => {
                const planId = Number(value)
                setSelectedPlanId(planId)
                applyPlanToForm(planId)
              }}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='یک پلن انتخاب کنید (اختیاری)' />
              </SelectTrigger>
              <SelectContent>
                {activeDnsPlans.map((plan) => (
                  <SelectItem key={plan.id} value={String(plan.id)}>
                    {plan.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className='space-y-2 rounded-lg border p-4'>
          <div className='flex items-center justify-between gap-3'>
            <div>
              <p className='text-sm font-medium'>پلن</p>
              <p className='text-muted-foreground text-sm'>
                پلن این حساب روی پنل doctor-dns -- فهرست پلن‌ها را با دکمه‌ی
                کنار بارگذاری کنید.
              </p>
            </div>
            <button
              type='button'
              onClick={handleLoadTemplates}
              disabled={isTestPending || isTestForResellerPending}
              className='text-primary text-sm font-medium whitespace-nowrap hover:underline disabled:opacity-50'
            >
              {isTestPending || isTestForResellerPending
                ? 'در حال بارگذاری...'
                : 'بارگذاری پلن‌ها'}
            </button>
          </div>
          {templateOptions.length > 0 && (
            <Select
              value={templateId ? String(templateId) : undefined}
              onValueChange={(value) => setTemplateId(Number(value))}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='یک پلن انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {templateOptions.map((template) => (
                  <SelectItem key={template.id} value={String(template.id)}>
                    {template.name}
                    {template.is_default ? ' (پیش‌فرض)' : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className='space-y-2 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>کشورهای مجاز</p>
            <p className='text-muted-foreground text-sm'>
              اگر هیچ کشوری انتخاب نشود، این حساب بدون محدودیت جغرافیایی
              است. در غیر این صورت، ثبت IP فقط از کشورهای تیک‌خورده در زیر
              پذیرفته می‌شود.
            </p>
          </div>
          <div className='grid max-h-56 gap-2 overflow-y-auto md:grid-cols-3'>
            {COMMON_COUNTRIES.map((country) => (
              <label key={country} className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={allowedCountries.includes(country)}
                  onCheckedChange={(checked) =>
                    toggleCountry(country, Boolean(checked))
                  }
                />
                {country}
              </label>
            ))}
          </div>
        </div>
      </form>
    </Form>
  )
}
