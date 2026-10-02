'use client'

import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import {
  Application,
  ApplicationGroupProfileSchema,
  UpdateApplicationRequest,
  UpdateApplicationSchema,
} from '@/schema/application.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { XuiPanel } from '@/schema/xui-panel.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useUpdateApplicationMutation } from '@/hooks/applications/useUpdateApplicationMutation.ts'
import { useUpdateApplicationForResellerMutation } from '@/hooks/applications/useUpdateApplicationForResellerMutation.ts'
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
  disabled: boolean
  download_speed_limit_mbps: number | string
  upload_speed_limit_mbps: number | string
}

interface Props {
  currentRow: Application
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is editing this Application on behalf of a
  // specific reseller (admin "Resellers Applications" page) -- mirrors
  // ApplicationForm's own targetResellerId convention exactly.
  targetResellerId?: number
  formId?: string
}

export function ApplicationEditForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
  formId = 'application-edit-form',
}: Props) {
  const isForReseller = targetResellerId !== undefined

  const form = useForm<FormValues>({
    defaultValues: {
      name: currentRow.name,
      total_volume_gb: Number(
        (currentRow.total_volume_bytes / BYTES_PER_GB).toFixed(2)
      ),
      duration_days: currentRow.duration_days,
      max_online_users: currentRow.max_online_users,
      disabled: currentRow.disabled,
      download_speed_limit_mbps: currentRow.download_speed_limit_mbps ?? '',
      upload_speed_limit_mbps: currentRow.upload_speed_limit_mbps ?? '',
    },
  })

  const { mutateAsync: updateApp, isPending: isUpdatePending } =
    useUpdateApplicationMutation()
  const {
    mutateAsync: updateAppForReseller,
    isPending: isUpdateForResellerPending,
  } = useUpdateApplicationForResellerMutation()
  const isPending = isForReseller
    ? isUpdateForResellerPending
    : isUpdatePending

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  const authRole = useAuthStore((state) => state.auth.admin?.role)
  const authResellerId = useAuthStore((state) => state.auth.admin?.reseller_id)
  const isReseller = authRole === 'reseller'
  const scopedResellerId =
    targetResellerId ?? (isReseller ? (authResellerId ?? undefined) : undefined)

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

  // User Manager groups/profiles
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

  // Seeded from currentRow's OWN current resources -- never "select all
  // candidates" the way create defaults to, since that would silently
  // ADD every other available interface/group/panel to an existing
  // Application the moment the edit dialog opens. Re-seeds only when the
  // row being edited changes (currentRow.id), not on every candidate-list
  // reference change, so toggling a checkbox isn't fought on next render.
  const [selectedInterfaceIds, setSelectedInterfaceIds] = useState<number[]>(
    () => currentRow.wireguard_peers.map((r) => r.resource_id)
  )
  const [selectedGroups, setSelectedGroups] = useState<string[]>(() =>
    currentRow.user_manager_accounts.map((r) => r.resource_name)
  )
  const [groupProfiles, setGroupProfiles] = useState<Record<string, string>>(
    () =>
      Object.fromEntries(
        currentRow.user_manager_accounts.map((r) => [
          r.resource_name,
          r.profile_name ?? '',
        ])
      )
  )
  const [selectedPanelIds, setSelectedPanelIds] = useState<number[]>(() =>
    currentRow.v2ray_packages.map((r) => r.resource_id)
  )

  useEffect(() => {
    setSelectedInterfaceIds(currentRow.wireguard_peers.map((r) => r.resource_id))
    setSelectedGroups(currentRow.user_manager_accounts.map((r) => r.resource_name))
    setGroupProfiles(
      Object.fromEntries(
        currentRow.user_manager_accounts.map((r) => [
          r.resource_name,
          r.profile_name ?? '',
        ])
      )
    )
    setSelectedPanelIds(currentRow.v2ray_packages.map((r) => r.resource_id))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentRow.id])

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

    // Omitting these two fields leaves the stored value untouched (see
    // the backend's own UpdateApplicationRequest doc comment) -- so an
    // emptied input here is only ever validated, never sent as an
    // explicit "clear back to unlimited" (that's a known, pre-existing
    // limitation shared with every other update field in this form).
    let downloadSpeedLimitMbps: number | undefined
    if (values.download_speed_limit_mbps !== '' && values.download_speed_limit_mbps != null) {
      downloadSpeedLimitMbps = Number(values.download_speed_limit_mbps)
      if (!Number.isFinite(downloadSpeedLimitMbps) || downloadSpeedLimitMbps <= 0) {
        toast.error('محدودیت سرعت دانلود باید یک عدد مثبت باشد.', { duration: 5000 })
        return
      }
    }
    let uploadSpeedLimitMbps: number | undefined
    if (values.upload_speed_limit_mbps !== '' && values.upload_speed_limit_mbps != null) {
      uploadSpeedLimitMbps = Number(values.upload_speed_limit_mbps)
      if (!Number.isFinite(uploadSpeedLimitMbps) || uploadSpeedLimitMbps <= 0) {
        toast.error('محدودیت سرعت آپلود باید یک عدد مثبت باشد.', { duration: 5000 })
        return
      }
    }

    if (
      selectedInterfaceIds.length === 0 &&
      selectedGroups.length === 0 &&
      selectedPanelIds.length === 0
    ) {
      toast.error('حداقل یک پروتکل/منبع باید انتخاب شود.', { duration: 5000 })
      return
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
      const payload: UpdateApplicationRequest = {
        id: currentRow.id,
        ...UpdateApplicationSchema.parse({
          name: values.name,
          total_volume_bytes: totalVolumeBytes,
          duration_days: durationDays,
          max_online_users: maxOnlineUsers,
          disabled: values.disabled,
          download_speed_limit_mbps: downloadSpeedLimitMbps,
          upload_speed_limit_mbps: uploadSpeedLimitMbps,
          interface_ids: selectedInterfaceIds,
          user_manager_groups: userManagerGroups,
          xui_panel_ids: selectedPanelIds,
        }),
      }
      if (isForReseller) {
        await updateAppForReseller({ resellerId: targetResellerId, app: payload })
      } else {
        await updateApp(payload)
      }
      toast.success('اپلیکیشن به‌روزرسانی شد.', { duration: 5000 })
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'به‌روزرسانی ناموفق بود. دوباره تلاش کنید.')
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
                <Input {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

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
            V2Ray/SSTP), not a MikroTik queue. Empty means unlimited (or,
            for an already-set value, "leave unchanged" on update -- see
            this form's own onSubmit doc comment). */}
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

        {/* Editing these checkboxes reconciles this Application's real
            underlying resources: unchecking removes that peer/account/
            package, checking a new one provisions it -- see
            UpdateApplication's own reconcileApplicationInterfaces/
            reconcileApplicationUserManagerGroups/reconcileApplicationXuiPanels
            doc comments on the Go side for the full add/remove behavior
            this drives. */}
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

        <FormField
          control={form.control}
          name='disabled'
          render={({ field }) => (
            <FormItem className='flex flex-row items-center gap-2 space-y-0'>
              <FormControl>
                <Checkbox
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
              <FormLabel className='font-normal'>
                غیرفعال کردن دستی (دسترسی VPN قطع می‌شود، لاگین اپ همچنان کار می‌کند)
              </FormLabel>
            </FormItem>
          )}
        />
      </form>
    </Form>
  )
}
