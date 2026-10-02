'use client'

import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import {
  CreateV2RayPackageRequest,
  CreateV2RayPackageSchema,
  UpdateV2RayPackageRequest,
  UpdateV2RayPackageSchema,
  V2RayPackage,
  V2RayPackageStatusEnum,
} from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { XuiPanel } from '@/schema/xui-panel.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateV2RayPackageMutation } from '@/hooks/v2ray/useCreateV2RayPackageMutation.ts'
import { useUpdateV2RayPackageMutation } from '@/hooks/v2ray/useUpdateV2RayPackageMutation.ts'
import { useCreateV2RayPackageForResellerMutation } from '@/hooks/v2ray/useCreateV2RayPackageForResellerMutation.ts'
import { useUpdateV2RayPackageForResellerMutation } from '@/hooks/v2ray/useUpdateV2RayPackageForResellerMutation.ts'
import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { useAssignedXuiPanelSummariesQuery } from '@/hooks/v2ray/useAssignedXuiPanelSummariesQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
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

// A module-level, genuinely stable empty array -- NOT an inline `= []`
// default in a destructure (`const { data: allPanels = [] } = ...`),
// which creates a BRAND NEW array reference on every single render
// whenever `data` is undefined (e.g. while the query is still loading).
// This was a confirmed, reported bug: that fresh reference fed
// candidatePanels' own useMemo below, which fed a useEffect keyed on
// candidatePanels -- so while allPanels was still loading, every render
// produced a new candidatePanels array, which retriggered the effect,
// which called setSelectedPanelIds, which triggered another render,
// forever -- React's own "Maximum update depth exceeded" infinite-loop
// error, which crashed the whole page to the app-wide error boundary the
// INSTANT the Edit dialog opened (before any field was even touched).
const EMPTY_PANELS: XuiPanel[] = []

// candidatePanels (below) can come from either the admin-only full-panel
// list (XuiPanel, with credentials) or the reseller-safe assigned-summary
// endpoint (XuiPanelSummary, id+name only) -- the checkbox picker JSX only
// ever reads .id/.name, so this minimal shared shape is all it needs.
type PanelPickerEntry = { id: number; name: string }
const EMPTY_PANEL_SUMMARIES: PanelPickerEntry[] = []

interface FormValues {
  id?: number
  customer_label: string
  comment: string
  total_volume_gb: number | string
  duration_days: number | string
  status?: 'active' | 'suspended' | 'expired'
}

interface Props {
  currentRow?: V2RayPackage
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is creating/editing this package on behalf of a
  // specific reseller (admin "Resellers V2Ray" page).
  targetResellerId?: number
  formId?: string
}

export function V2RayForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
  formId = 'v2ray-package-form',
}: Props) {
  const isEdit = !!currentRow
  const isForReseller = targetResellerId !== undefined

  const form = useForm<FormValues>({
    defaultValues: isEdit
      ? {
          id: currentRow.id,
          customer_label: currentRow.customer_label ?? '',
          comment: currentRow.comment ?? '',
          total_volume_gb: Number(
            (currentRow.total_volume_bytes / BYTES_PER_GB).toFixed(2)
          ),
          duration_days: currentRow.duration_days,
          status: currentRow.status,
        }
      : {
          customer_label: '',
          comment: '',
          total_volume_gb: '',
          duration_days: '',
        },
  })

  const { mutateAsync: createPkg, isPending: isCreatePending } =
    useCreateV2RayPackageMutation()
  const { mutateAsync: updatePkg, isPending: isUpdatePending } =
    useUpdateV2RayPackageMutation()
  const {
    mutateAsync: createPkgForReseller,
    isPending: isCreateForResellerPending,
  } = useCreateV2RayPackageForResellerMutation()
  const {
    mutateAsync: updatePkgForReseller,
    isPending: isUpdateForResellerPending,
  } = useUpdateV2RayPackageForResellerMutation()

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

  // Panel selection only matters on create -- CreatePackage is the only
  // call that fans out to panels; UpdatePackage never touches locations
  // (see the Go service's own doc comment), so an edit never shows this
  // picker at all.
  const authRole = useAuthStore((state) => state.auth.admin?.role)
  const authResellerId = useAuthStore((state) => state.auth.admin?.reseller_id)

  // Candidate set: every registered panel for an admin-direct package,
  // or only the panels the relevant reseller has been explicitly granted
  // otherwise -- targetResellerId when an admin is acting on a reseller's
  // behalf, the logged-in reseller's own id when they're creating for
  // themselves.
  const scopedResellerId =
    targetResellerId ?? (authRole === 'reseller' ? (authResellerId ?? undefined) : undefined)

  // GET /api/xui-panel (the full panel list, with connection credentials)
  // is admin-only server-side -- only fetch it when this form is actually
  // being used in an admin-direct context (scopedResellerId === undefined).
  // A logged-in reseller instead relies entirely on
  // useAssignedXuiPanelSummariesQuery below, a reseller-permitted endpoint
  // that resolves their assigned panel IDs into id+name pairs directly, so
  // this component never needs the admin-only list to render a reseller's
  // picker. See useXuiPanelsListQuery's own doc comment: fetching this
  // unconditionally for a reseller session was a confirmed, reported bug --
  // the resulting 403 tripped the app's global query-error handler, which
  // redirected the whole page to /403 the instant "Add Package" was opened.
  //
  // EMPTY_PANELS (module-level constant), NOT an inline `= []` default --
  // see its own doc comment for why that was a separate, confirmed,
  // reported infinite-render-loop bug.
  const { data: allPanels = EMPTY_PANELS } = useXuiPanelsListQuery(
    scopedResellerId === undefined
  )
  const { data: assignedPanelSummaries } =
    useAssignedXuiPanelSummariesQuery(scopedResellerId)

  const candidatePanels: PanelPickerEntry[] = useMemo(() => {
    if (scopedResellerId === undefined) {
      return allPanels
    }
    return assignedPanelSummaries ?? EMPTY_PANEL_SUMMARIES
  }, [allPanels, assignedPanelSummaries, scopedResellerId])

  const [selectedPanelIds, setSelectedPanelIds] = useState<number[]>([])

  // Smart default on CREATE: pre-select every candidate panel whenever the
  // candidate set changes (covers both "only one panel exists, use it
  // automatically" and "multiple panels exist, default to all"). On EDIT,
  // seed from the package's OWN current locations instead -- defaulting to
  // "all candidate panels" here would silently ADD every other panel the
  // admin/reseller can use to an existing package the moment they open the
  // edit dialog, which is not what "edit this package" should ever do.
  // Only re-seeds when the row being edited changes (currentRow?.id), not
  // on every candidatePanels reference change, so toggling a checkbox
  // isn't fought by this effect on the next render.
  useEffect(() => {
    if (isEdit && currentRow) {
      setSelectedPanelIds(currentRow.locations.map((loc) => loc.panel_id))
      return
    }
    setSelectedPanelIds(candidatePanels.map((panel) => panel.id))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isEdit, currentRow?.id, candidatePanels])

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

    if (!Number.isFinite(totalVolumeBytes) || totalVolumeBytes <= 0) {
      toast.error('حجم کل باید عددی مثبت بر حسب گیگابایت باشد.', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(durationDays) || durationDays <= 0) {
      toast.error('مدت زمان باید عددی مثبت بر حسب روز باشد.', {
        duration: 5000,
      })
      return
    }
    if (candidatePanels.length > 0 && selectedPanelIds.length === 0) {
      toast.error('حداقل یک سرور x-ui برای این بسته انتخاب کنید.', {
        duration: 5000,
      })
      return
    }

    // A confirmed, reported bug: this whole block previously had NO
    // try/catch. If the schema parse or the server round-trip threw for
    // ANY reason (a validation error, a not-found package, an actual
    // backend 500, or the response Zod-parse itself failing), the
    // rejection was completely unhandled -- which crashed the entire page
    // to TanStack Router's app-wide error boundary (the generic "500 --
    // Oops! Something went wrong" screen), instead of showing a normal
    // toast the way every other form in this codebase does. This is why
    // it looked like "a 500 error, sometimes" -- the underlying cause was
    // often a perfectly ordinary, recoverable error, but the missing
    // try/catch turned it into a full page crash.
    try {
      if (isForReseller) {
        if (isEdit) {
          const payload: UpdateV2RayPackageRequest =
            UpdateV2RayPackageSchema.parse({
              id: currentRow.id,
              customer_label: values.customer_label || null,
              comment: values.comment || null,
              total_volume_bytes: totalVolumeBytes,
              duration_days: durationDays,
              status: values.status,
              panel_ids: selectedPanelIds,
            })
          await updatePkgForReseller({
            resellerId: targetResellerId,
            pkg: payload,
          })
          toast.success('بسته با موفقیت به‌روزرسانی شد.', { duration: 5000 })
        } else {
          const payload: CreateV2RayPackageRequest =
            CreateV2RayPackageSchema.parse({
              customer_label: values.customer_label || null,
              comment: values.comment || null,
              total_volume_bytes: totalVolumeBytes,
              duration_days: durationDays,
              panel_ids: selectedPanelIds,
            })
          await createPkgForReseller({
            resellerId: targetResellerId,
            pkg: payload,
          })
          toast.success('بسته با موفقیت ایجاد شد.', { duration: 5000 })
        }
      } else if (isEdit) {
        const payload: UpdateV2RayPackageRequest =
          UpdateV2RayPackageSchema.parse({
            id: currentRow.id,
            customer_label: values.customer_label || null,
            total_volume_bytes: totalVolumeBytes,
            duration_days: durationDays,
            status: values.status,
            panel_ids: selectedPanelIds,
          })
        await updatePkg(payload)
        toast.success('بسته با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        const payload: CreateV2RayPackageRequest =
          CreateV2RayPackageSchema.parse({
            customer_label: values.customer_label || null,
            total_volume_bytes: totalVolumeBytes,
            duration_days: durationDays,
            panel_ids: selectedPanelIds,
          })
        await createPkg(payload)
        toast.success('بسته با موفقیت ایجاد شد.', { duration: 5000 })
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره‌ی بسته ناموفق بود. دوباره تلاش کنید.')
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
                    نمی‌شود و در محتوای سابسکریپشن/ربات گنجانده نمی‌شود.
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
                    min='0.01'
                    placeholder='مثلاً 50'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  به‌صورت خودکار به بایت تبدیل می‌شود.
                </FormDescription>
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
                    min='1'
                    placeholder='مثلاً 30'
                    value={field.value ?? ''}
                    onChange={(e) => field.onChange(e.target.value)}
                  />
                </FormControl>
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
                      {V2RayPackageStatusEnum.options.map((status) => (
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

        {/* No picker at all when there's only one candidate panel -- it's
            used automatically either way. Shown on both create and edit:
            unchecking a panel here on an existing package disables its
            client there and removes that location; checking a
            previously-unselected panel adds a brand new client on it --
            see UpdatePackage's own reconcilePackageLocations doc comment
            on the Go side for the full add/remove behavior this drives. */}
        {candidatePanels.length > 1 && (
          <div className='space-y-2 rounded-lg border p-4'>
            <div>
              <p className='text-sm font-medium'>سرورهای X-UI</p>
              <p className='text-muted-foreground text-sm'>
                {isEdit
                  ? 'این بسته روی هر سروری که در زیر تیک خورده وجود دارد. برداشتن تیک یک سرور، این بسته را از آن حذف می‌کند؛ تیک زدن یک سرور جدید، بسته را به آن اضافه می‌کند.'
                  : 'این بسته روی هر سروری که در زیر تیک خورده ایجاد خواهد شد. به‌طور پیش‌فرض روی همه‌ی سرورهایی که می‌توانید استفاده کنید فعال است.'}
              </p>
            </div>
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

        {!isEdit && candidatePanels.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {scopedResellerId !== undefined
              ? 'هنوز هیچ سرور x-ui به این نماینده اختصاص داده نشده است -- با ادمین خود تماس بگیرید.'
              : 'هنوز هیچ سرور x-ui ثبت نشده است. ابتدا یکی را در بخش سرور X-UI اضافه کنید.'}
          </p>
        )}
      </form>
    </Form>
  )
}
