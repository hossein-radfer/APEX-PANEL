'use client'

import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import {
  Application,
  ApplicationGroupProfileSchema,
  CreateApplicationRequest,
  CreateApplicationSchema,
} from '@/schema/application.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { XuiPanel } from '@/schema/xui-panel.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateApplicationMutation } from '@/hooks/applications/useCreateApplicationMutation.ts'
import { useCreateApplicationForResellerMutation } from '@/hooks/applications/useCreateApplicationForResellerMutation.ts'
import { useApplicationPlansListQuery } from '@/hooks/application-plan/useApplicationPlansListQuery.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'
import { useAssignedInterfacesQuery } from '@/hooks/resellers/useAssignedInterfacesQuery.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
import { useAssignedUserManagerGroupsQuery } from '@/hooks/resellers/useAssignedUserManagerGroupsQuery.ts'
import { useAssignedUserManagerProfilesQuery } from '@/hooks/resellers/useAssignedUserManagerProfilesQuery.ts'
import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { useAssignedXuiPanelSummariesQuery } from '@/hooks/v2ray/useAssignedXuiPanelSummariesQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import {
  Form,
  FormControl,
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

const EMPTY_PANELS: XuiPanel[] = []
type PanelPickerEntry = { id: number; name: string }
const EMPTY_PANEL_SUMMARIES: PanelPickerEntry[] = []

interface FormValues {
  name: string
  total_volume_gb: number | string
  duration_days: number | string
  max_online_users: number | string
  app_username: string
  app_password: string
  download_speed_limit_mbps: number | string
  upload_speed_limit_mbps: number | string
}

interface Props {
  // onCreated fires right after a successful create, with the newly
  // created Application -- see ApplicationCredentialsDialog's own doc
  // comment for why the create flow no longer closes itself directly:
  // the admin needs a chance to actually see/copy the generated
  // app_username/app_password first. The caller (ApplicationDialogs)
  // owns showing that follow-up dialog and closing this one.
  onCreated: (application: Application) => void
  setIsLoading?: (loading: boolean) => void
  targetResellerId?: number
  formId?: string
}

export function ApplicationForm({
  onCreated,
  setIsLoading,
  targetResellerId,
  formId = 'application-form',
}: Props) {
  const isForReseller = targetResellerId !== undefined

  const form = useForm<FormValues>({
    defaultValues: {
      name: '',
      total_volume_gb: '',
      duration_days: '',
      max_online_users: 1,
      app_username: '',
      app_password: '',
      download_speed_limit_mbps: '',
      upload_speed_limit_mbps: '',
    },
  })

  const { mutateAsync: createApp, isPending: isCreatePending } =
    useCreateApplicationMutation()
  const {
    mutateAsync: createAppForReseller,
    isPending: isCreateForResellerPending,
  } = useCreateApplicationForResellerMutation()

  const isPending = isForReseller ? isCreateForResellerPending : isCreatePending

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  const authRole = useAuthStore((state) => state.auth.admin?.role)
  const authResellerId = useAuthStore((state) => state.auth.admin?.reseller_id)
  const isReseller = authRole === 'reseller'
  const scopedResellerId =
    targetResellerId ?? (isReseller ? (authResellerId ?? undefined) : undefined)

  // Application tier catalog -- reachable by both admin and reseller
  // sessions (see useApplicationPlansListQuery's own doc comment), unlike
  // DNS's admin-only plan list. Mirrors DNSAccountForm's identical
  // selectedPlanId/applyPlanToForm pattern.
  const { data: applicationPlans } = useApplicationPlansListQuery()
  const [selectedPlanId, setSelectedPlanId] = useState<number | undefined>(
    undefined
  )
  const applyPlanToForm = (planId: number) => {
    const plan = (applicationPlans ?? []).find(
      (candidate) => candidate.id === planId
    )
    if (!plan) return
    form.setValue(
      'total_volume_gb',
      plan.total_volume_bytes > 0
        ? Number((plan.total_volume_bytes / BYTES_PER_GB).toFixed(2))
        : 0
    )
    form.setValue('duration_days', plan.duration_days)
    form.setValue('max_online_users', plan.max_online_users)
    if (plan.download_speed_limit_mbps) {
      form.setValue('download_speed_limit_mbps', plan.download_speed_limit_mbps)
    }
    if (plan.upload_speed_limit_mbps) {
      form.setValue('upload_speed_limit_mbps', plan.upload_speed_limit_mbps)
    }
  }

  // WireGuard interfaces
  const { data: allInterfaces = [] } = useInterfacesListQuery(
    scopedResellerId === undefined
  )
  const { data: assignedInterfaceIds } =
    useAssignedInterfacesQuery(scopedResellerId)
  const candidateInterfaces = useMemo(() => {
    if (scopedResellerId === undefined) return allInterfaces
    const allowed = new Set(assignedInterfaceIds ?? [])
    return allInterfaces.filter((iface) => allowed.has(iface.id))
  }, [allInterfaces, assignedInterfaceIds, scopedResellerId])

  // User Manager groups/profiles -- useUserManagerGroupsQuery/
  // useUserManagerProfilesQuery return {name}[] objects (they carry no
  // other fields RouterOS-side), immediately flattened to plain string[]
  // here since every consumer below (candidateGroups/candidateProfiles,
  // selectedGroups, groupProfiles) only ever needs the bare name.
  const { data: allGroupObjects = [] } = useUserManagerGroupsQuery()
  const { data: allProfileObjects = [] } = useUserManagerProfilesQuery()
  const allGroups = useMemo(
    () => allGroupObjects.map((g) => g.name),
    [allGroupObjects]
  )
  const allProfiles = useMemo(
    () => allProfileObjects.map((p) => p.name),
    [allProfileObjects]
  )
  const { data: assignedGroups } = useAssignedUserManagerGroupsQuery(
    scopedResellerId
  )
  const { data: assignedProfiles } = useAssignedUserManagerProfilesQuery(
    scopedResellerId
  )
  const candidateGroups = useMemo(() => {
    if (scopedResellerId === undefined) return allGroups
    const allowed = new Set(assignedGroups ?? [])
    return allGroups.filter((g) => allowed.has(g))
  }, [allGroups, assignedGroups, scopedResellerId])
  const candidateProfiles = useMemo(() => {
    if (scopedResellerId === undefined) return allProfiles
    const allowed = new Set(assignedProfiles ?? [])
    return allProfiles.filter((p) => allowed.has(p))
  }, [allProfiles, assignedProfiles, scopedResellerId])

  // V2Ray xui panels
  const { data: allPanels = EMPTY_PANELS } = useXuiPanelsListQuery(
    scopedResellerId === undefined
  )
  const { data: assignedPanelSummaries } =
    useAssignedXuiPanelSummariesQuery(scopedResellerId)
  const candidatePanels: PanelPickerEntry[] = useMemo(() => {
    if (scopedResellerId === undefined) return allPanels
    return assignedPanelSummaries ?? EMPTY_PANEL_SUMMARIES
  }, [allPanels, assignedPanelSummaries, scopedResellerId])

  const [selectedInterfaceIds, setSelectedInterfaceIds] = useState<number[]>([])
  const [selectedGroups, setSelectedGroups] = useState<string[]>([])
  const [groupProfiles, setGroupProfiles] = useState<Record<string, string>>({})
  const [selectedPanelIds, setSelectedPanelIds] = useState<number[]>([])

  const toggleInterface = (id: number, checked: boolean) => {
    setSelectedInterfaceIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }
  const toggleGroup = (group: string, checked: boolean) => {
    setSelectedGroups((prev) =>
      checked ? [...prev, group] : prev.filter((g) => g !== group)
    )
    if (!checked) {
      setGroupProfiles((prev) => {
        const next = { ...prev }
        delete next[group]
        return next
      })
    }
  }
  const togglePanel = (id: number, checked: boolean) => {
    setSelectedPanelIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }

  const onSubmit = async (values: FormValues) => {
    const totalVolumeBytes = Math.round(
      Number(values.total_volume_gb) * BYTES_PER_GB
    )
    const durationDays = Number(values.duration_days)
    const maxOnlineUsers = Number(values.max_online_users)

    if (!Number.isFinite(totalVolumeBytes) || totalVolumeBytes <= 0) {
      toast.error('حجم کل باید یک عدد مثبت (گیگابایت) باشد.', { duration: 5000 })
      return
    }
    if (!Number.isFinite(durationDays) || durationDays <= 0) {
      toast.error('مدت باید یک عدد مثبت (روز) باشد.', { duration: 5000 })
      return
    }
    if (!Number.isFinite(maxOnlineUsers) || maxOnlineUsers <= 0) {
      toast.error('تعداد کاربر آنلاین مجاز باید حداقل ۱ باشد.', {
        duration: 5000,
      })
      return
    }
    if (
      selectedInterfaceIds.length === 0 &&
      selectedGroups.length === 0 &&
      selectedPanelIds.length === 0
    ) {
      toast.error('حداقل یک پروتکل/منبع باید انتخاب شود.', { duration: 5000 })
      return
    }

    let downloadSpeedLimitMbps: number | null = null
    if (values.download_speed_limit_mbps !== '' && values.download_speed_limit_mbps != null) {
      downloadSpeedLimitMbps = Number(values.download_speed_limit_mbps)
      if (!Number.isFinite(downloadSpeedLimitMbps) || downloadSpeedLimitMbps <= 0) {
        toast.error('محدودیت سرعت دانلود باید یک عدد مثبت باشد.', { duration: 5000 })
        return
      }
    }
    let uploadSpeedLimitMbps: number | null = null
    if (values.upload_speed_limit_mbps !== '' && values.upload_speed_limit_mbps != null) {
      uploadSpeedLimitMbps = Number(values.upload_speed_limit_mbps)
      if (!Number.isFinite(uploadSpeedLimitMbps) || uploadSpeedLimitMbps <= 0) {
        toast.error('محدودیت سرعت آپلود باید یک عدد مثبت باشد.', { duration: 5000 })
        return
      }
    }

    const userManagerGroups = selectedGroups.map((group) => ({
      group_name: group,
      profile_name: groupProfiles[group] ?? '',
    }))
    for (const g of userManagerGroups) {
      const parsed = ApplicationGroupProfileSchema.safeParse(g)
      if (!parsed.success) {
        toast.error(`برای گروه «${g.group_name}» باید یک پروفایل انتخاب شود.`, {
          duration: 5000,
        })
        return
      }
    }

    try {
      const payload: CreateApplicationRequest = CreateApplicationSchema.parse({
        name: values.name,
        interface_ids: selectedInterfaceIds,
        user_manager_groups: userManagerGroups,
        xui_panel_ids: selectedPanelIds,
        application_plan_id: selectedPlanId ?? null,
        total_volume_bytes: totalVolumeBytes,
        duration_days: durationDays,
        max_online_users: maxOnlineUsers,
        app_username: values.app_username || null,
        app_password: values.app_password || null,
        download_speed_limit_mbps: downloadSpeedLimitMbps,
        upload_speed_limit_mbps: uploadSpeedLimitMbps,
      })

      const created = isForReseller
        ? await createAppForReseller({
            resellerId: targetResellerId,
            app: payload,
          })
        : await createApp(payload)

      if (created.provisioning_errors && created.provisioning_errors.length > 0) {
        // Some requested resources failed to provision (e.g. an IP-pool
        // collision on one specific WireGuard interface) -- the
        // Application itself was still created with whatever succeeded,
        // so this must be surfaced clearly rather than shown as a plain
        // success, otherwise the admin has no way to know a checked
        // protocol silently didn't get set up.
        toast.error(
          `اپلیکیشن ساخته شد، ولی برخی منابع تأمین نشدند:\n${created.provisioning_errors.join('\n')}`,
          { duration: 10000 }
        )
      } else {
        toast.success('اپلیکیشن با موفقیت ساخته شد.', { duration: 5000 })
      }
      form.reset()
      setSelectedPlanId(undefined)
      onCreated(created)
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ساخت اپلیکیشن ناموفق بود. دوباره تلاش کنید.')
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
        <FormField
          control={form.control}
          name='name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>نام اپلیکیشن</FormLabel>
              <FormControl>
                <Input placeholder='مثلاً: اپلیکیشن مشتری ۱' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Application tier catalog -- picking a plan pre-fills the
            volume/duration/online-user/speed fields below, which remain
            freely editable afterward (manual override). Mirrors
            DNSAccountForm's identical pricing-plan box. */}
        <div className='space-y-2 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>پلن اپلیکیشن</p>
            <p className='text-muted-foreground text-sm'>
              با انتخاب یک پلن، فیلدهای حجم/مدت/کاربر آنلاین/سرعت پایین از
              روی آن پر می‌شوند -- همچنان می‌توانید هرکدام را دستی تغییر
              دهید.
            </p>
          </div>
          {(applicationPlans ?? []).length === 0 ? (
            <p className='text-muted-foreground text-sm'>
              هنوز هیچ پلنی ثبت نشده است.
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
                {(applicationPlans ?? []).map((plan) => (
                  <SelectItem key={plan.id} value={String(plan.id)}>
                    {plan.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-3'>
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
                    min='0.01'
                    placeholder='مثلاً ۵۰'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='duration_days'
            render={({ field }) => (
              <FormItem>
                <FormLabel>مدت (روز)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    placeholder='مثلاً ۳۰'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='max_online_users'
            render={({ field }) => (
              <FormItem>
                <FormLabel>کاربر آنلاین مجاز</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* Client-side speed limit -- enforced by the mobile app itself,
            uniformly across every protocol it connects with (WireGuard/
            V2Ray/SSTP), not a MikroTik queue. Empty means unlimited. */}
        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='download_speed_limit_mbps'
            render={({ field }) => (
              <FormItem>
                <FormLabel>محدودیت سرعت دانلود (Mbps)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    placeholder='نامحدود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='upload_speed_limit_mbps'
            render={({ field }) => (
              <FormItem>
                <FormLabel>محدودیت سرعت آپلود (Mbps)</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    placeholder='نامحدود'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {candidateInterfaces.length > 0 && (
          <div className='space-y-2 rounded-lg border p-4'>
            <p className='text-sm font-medium'>اینترفیس‌های وایرگارد</p>
            <div className='grid gap-2 md:grid-cols-2'>
              {candidateInterfaces.map((iface) => (
                <label key={iface.id} className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={selectedInterfaceIds.includes(iface.id)}
                    onCheckedChange={(checked) =>
                      toggleInterface(iface.id, Boolean(checked))
                    }
                  />
                  {iface.name}
                </label>
              ))}
            </div>
          </div>
        )}

        {candidateGroups.length > 0 && (
          <div className='space-y-2 rounded-lg border p-4'>
            <p className='text-sm font-medium'>گروه‌های یوزرمنجیر</p>
            <div className='space-y-2'>
              {candidateGroups.map((group) => (
                <div key={group} className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={selectedGroups.includes(group)}
                    onCheckedChange={(checked) => toggleGroup(group, Boolean(checked))}
                  />
                  <span className='w-32 shrink-0'>{group}</span>
                  {selectedGroups.includes(group) && (
                    <Select
                      value={groupProfiles[group] ?? ''}
                      onValueChange={(value) =>
                        setGroupProfiles((prev) => ({ ...prev, [group]: value }))
                      }
                    >
                      <SelectTrigger className='h-8 flex-1'>
                        <SelectValue placeholder='انتخاب پروفایل' />
                      </SelectTrigger>
                      <SelectContent>
                        {candidateProfiles.map((profile) => (
                          <SelectItem key={profile} value={profile}>
                            {profile}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {candidatePanels.length > 0 && (
          <div className='space-y-2 rounded-lg border p-4'>
            <p className='text-sm font-medium'>پنل‌های V2Ray</p>
            <div className='grid gap-2 md:grid-cols-2'>
              {candidatePanels.map((panel) => (
                <label key={panel.id} className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={selectedPanelIds.includes(panel.id)}
                    onCheckedChange={(checked) =>
                      togglePanel(panel.id, Boolean(checked))
                    }
                  />
                  {panel.name}
                </label>
              ))}
            </div>
          </div>
        )}

        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='app_username'
            render={({ field }) => (
              <FormItem>
                <FormLabel>یوزرنیم اپ (اختیاری)</FormLabel>
                <FormControl>
                  <Input placeholder='خالی = رندوم' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='app_password'
            render={({ field }) => (
              <FormItem>
                <FormLabel>پسورد اپ (اختیاری)</FormLabel>
                <FormControl>
                  <Input placeholder='خالی = رندوم' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      </form>
    </Form>
  )
}
