import { useMemo, useState } from 'react'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { XuiPanel } from '@/schema/xui-panel.ts'
import { DownloadIcon, PackagePlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useBulkCreateV2RayPackagesMutation } from '@/hooks/v2ray/useBulkCreateV2RayPackagesMutation.ts'
import { useExportV2RayPackagesMutation } from '@/hooks/v2ray/useExportV2RayPackagesMutation.ts'
import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { useAssignedXuiPanelSummariesQuery } from '@/hooks/v2ray/useAssignedXuiPanelSummariesQuery.ts'
import { useResellerQuery } from '@/hooks/resellers/useResellerQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Button } from '@/components/ui/button.tsx'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'

// Same minimal shape v2ray-form.tsx's own picker uses -- see that file's
// PanelPickerEntry doc comment for why this is intentionally id+name only.
type PanelPickerEntry = { id: number; name: string }
const EMPTY_PANELS: XuiPanel[] = []
const EMPTY_PANEL_SUMMARIES: PanelPickerEntry[] = []

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
}

// V2RayBulkCreateDialog implements the admin's own explicit item #11
// requirement: create N packages at once (random name + volume + duration),
// then export the batch as .xlsx/.txt with independently-checkable
// share/subscription link columns. This is a single create-then-export flow
// -- there is no existing row-selection on the packages table to build a
// separate "export already-existing packages" action off of (see
// v2ray-columns.tsx), so exporting only ever covers a batch just created
// here, not arbitrary pre-existing packages.
export function V2RayBulkCreateDialog({ open, onOpenChange }: Props) {
  const [count, setCount] = useState<number | string>(10)
  const [volumeGb, setVolumeGb] = useState<number | string>('')
  const [durationDays, setDurationDays] = useState<number | string>('')
  const [selectedPanelIds, setSelectedPanelIds] = useState<number[]>([])
  const [includeShareLink, setIncludeShareLink] = useState(true)
  const [includeSubscriptionLink, setIncludeSubscriptionLink] = useState(true)
  const [format, setFormat] = useState<'xlsx' | 'txt'>('xlsx')

  const authRole = useAuthStore((state) => state.auth.admin?.role)
  const authResellerId = useAuthStore((state) => state.auth.admin?.reseller_id)
  const isReseller = authRole === 'reseller'

  const { data: selfReseller } = useResellerQuery(
    isReseller ? authResellerId : null
  )

  // Admin-direct (scopedResellerId undefined) sees every registered panel;
  // a reseller only ever sees their own assigned panels -- mirrors
  // v2ray-form.tsx's own candidatePanels resolution exactly.
  const scopedResellerId = isReseller ? (authResellerId ?? undefined) : undefined

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

  const togglePanel = (id: number, checked: boolean) => {
    setSelectedPanelIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }

  const bulkCreate = useBulkCreateV2RayPackagesMutation()
  const exportPackages = useExportV2RayPackagesMutation()

  const isPending = bulkCreate.isPending || exportPackages.isPending

  const resetForm = () => {
    setCount(10)
    setVolumeGb('')
    setDurationDays('')
    setSelectedPanelIds([])
  }

  const handleClose = (isOpen: boolean) => {
    onOpenChange(isOpen)
    if (!isOpen) {
      setTimeout(resetForm, 500)
    }
  }

  const handleSubmit = async () => {
    const parsedCount = Number(count)
    const totalVolumeBytes = Math.round(Number(volumeGb) * BYTES_PER_GB)
    const parsedDuration = Number(durationDays)

    if (!Number.isFinite(parsedCount) || parsedCount < 1 || parsedCount > 500) {
      toast.error('تعداد باید عددی بین 1 و 500 باشد.', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(totalVolumeBytes) || totalVolumeBytes <= 0) {
      toast.error('حجم کل باید عددی مثبت بر حسب گیگابایت باشد.', {
        duration: 5000,
      })
      return
    }
    if (!Number.isFinite(parsedDuration) || parsedDuration <= 0) {
      toast.error('مدت زمان باید عددی مثبت بر حسب روز باشد.', {
        duration: 5000,
      })
      return
    }
    if (candidatePanels.length > 0 && selectedPanelIds.length === 0) {
      toast.error('حداقل یک سرور x-ui برای ایجاد این بسته‌ها انتخاب کنید.', {
        duration: 5000,
      })
      return
    }
    if (!includeShareLink && !includeSubscriptionLink) {
      toast.error('حداقل یک نوع لینک برای گنجاندن در خروجی انتخاب کنید.', {
        duration: 5000,
      })
      return
    }

    try {
      const created = await bulkCreate.mutateAsync({
        count: parsedCount,
        total_volume_bytes: totalVolumeBytes,
        duration_days: parsedDuration,
        panel_ids: selectedPanelIds,
      })

      await exportPackages.mutateAsync({
        package_ids: created.map((pkg) => pkg.id),
        include_share_link: includeShareLink,
        include_subscription_link: includeSubscriptionLink,
        base_url: window.location.origin,
        format,
      })

      toast.success(
        `${created.length} بسته با موفقیت ایجاد و خروجی گرفته شد.`,
        { duration: 5000 }
      )
      handleClose(false)
    } catch (error) {
      toast.error(
        getApiErrorMessage(
          error,
          'ایجاد گروهی/خروجی گرفتن از بسته‌ها ناموفق بود. دوباره تلاش کنید.'
        )
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>ایجاد گروهی بسته</DialogTitle>
          <DialogDescription>
            چندین بسته را یک‌جا با نام‌های تصادفی ایجاد کنید، سپس از این
            دسته به‌صورت فایل اکسل یا متنی خروجی بگیرید.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-3'>
            <div className='space-y-2'>
              <Label>تعداد</Label>
              <Input
                type='number'
                step='1'
                min='1'
                max='500'
                placeholder='مثلاً 10'
                value={count}
                onChange={(e) => setCount(e.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label>حجم کل (گیگابایت)</Label>
              <Input
                type='number'
                step='0.01'
                min='0.01'
                placeholder='مثلاً 50'
                value={volumeGb}
                onChange={(e) => setVolumeGb(e.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label>مدت زمان (روز)</Label>
              <Input
                type='number'
                step='1'
                min='1'
                placeholder='مثلاً 30'
                value={durationDays}
                onChange={(e) => setDurationDays(e.target.value)}
              />
            </div>
          </div>

          {candidatePanels.length > 1 && (
            <div className='space-y-2 rounded-lg border p-4'>
              <div>
                <p className='text-sm font-medium'>سرورهای X-UI</p>
                <p className='text-muted-foreground text-sm'>
                  هر بسته در این دسته روی هر سروری که در زیر تیک خورده ایجاد
                  خواهد شد.
                </p>
              </div>
              <div className='grid gap-2 md:grid-cols-2'>
                {candidatePanels.map((panel) => (
                  <label
                    key={panel.id}
                    className='flex items-center gap-2 text-sm'
                  >
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

          {candidatePanels.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {scopedResellerId !== undefined
                ? 'هنوز هیچ سرور x-ui به این نماینده اختصاص داده نشده است -- با ادمین خود تماس بگیرید.'
                : 'هنوز هیچ سرور x-ui ثبت نشده است. ابتدا یکی را در بخش سرور X-UI اضافه کنید.'}
            </p>
          )}

          <div className='space-y-2 rounded-lg border p-4'>
            <p className='text-sm font-medium'>محتوای فایل خروجی</p>
            <label className='flex items-center gap-2 text-sm'>
              <Checkbox
                checked={includeShareLink}
                onCheckedChange={(checked) =>
                  setIncludeShareLink(Boolean(checked))
                }
              />
              شامل لینک صفحه‌ی اشتراک‌گذاری
            </label>
            <label className='flex items-center gap-2 text-sm'>
              <Checkbox
                checked={includeSubscriptionLink}
                onCheckedChange={(checked) =>
                  setIncludeSubscriptionLink(Boolean(checked))
                }
              />
              شامل لینک مستقیم سابسکریپشن
            </label>
          </div>

          <div className='space-y-2'>
            <Label>فرمت خروجی</Label>
            <Select
              value={format}
              onValueChange={(value) => setFormat(value as 'xlsx' | 'txt')}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='xlsx'>اکسل (.xlsx)</SelectItem>
                <SelectItem value='txt'>متنی (.txt)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {isReseller && !selfReseller?.canResellV2Ray && (
            <p className='text-destructive text-sm'>
              شما اجازه‌ی ایجاد بسته‌ی V2Ray را ندارید.
            </p>
          )}
        </div>

        <DialogFooter>
          <Button
            onClick={handleSubmit}
            disabled={
              isPending || (isReseller && !selfReseller?.canResellV2Ray)
            }
            className='gap-2'
          >
            {isPending ? (
              'در حال انجام...'
            ) : (
              <>
                <PackagePlusIcon className='h-4 w-4' />
                ایجاد و خروجی گرفتن
                <DownloadIcon className='h-4 w-4' />
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
