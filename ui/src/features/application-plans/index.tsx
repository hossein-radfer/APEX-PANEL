import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconEdit, IconPlus, IconTrash } from '@tabler/icons-react'
import {
  createApplicationPlan,
  deleteApplicationPlan,
  fetchAllApplicationPlans,
  updateApplicationPlan,
} from '@/api/application-plan.ts'
import { ApplicationPlan } from '@/schema/application-plan.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

// Admin-facing CRUD for phase 2's Application tier catalog (برنزی/نقره‌ای/
// طلایی) -- structurally mirrors DNSPlansPage exactly, adapted for
// Application's own bundle (no speed-in-Kbps/IP-limit concepts, instead
// MaxOnlineUsers + optional Mbps speed caps).

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

function gbToBytes(gb: string): number {
  return Math.round(Number.parseFloat(gb) * BYTES_PER_GB)
}

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

interface PlanFormState {
  name: string
  description: string
  priceAmount: string
  totalVolumeGb: string
  durationDays: string
  maxOnlineUsers: string
  downloadSpeedLimitMbps: string
  uploadSpeedLimitMbps: string
}

const emptyForm: PlanFormState = {
  name: '',
  description: '',
  priceAmount: '',
  totalVolumeGb: '',
  durationDays: '30',
  maxOnlineUsers: '1',
  downloadSpeedLimitMbps: '',
  uploadSpeedLimitMbps: '',
}

export default function ApplicationPlansPage() {
  const queryClient = useQueryClient()
  const { data: plans = [], isLoading } = useQuery({
    queryKey: ['application_plans'],
    queryFn: fetchAllApplicationPlans,
  })

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingPlan, setEditingPlan] = useState<ApplicationPlan | null>(null)
  const [form, setForm] = useState<PlanFormState>(emptyForm)
  const [deleteTarget, setDeleteTarget] = useState<ApplicationPlan | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['application_plans'] })

  const createMutation = useMutation({
    mutationFn: createApplicationPlan,
    onSuccess: () => {
      toast.success('پلن اپلیکیشن ایجاد شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ایجاد پلن ناموفق بود.')),
  })

  const updateMutation = useMutation({
    mutationFn: updateApplicationPlan,
    onSuccess: () => {
      toast.success('پلن اپلیکیشن به‌روزرسانی شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی پلن ناموفق بود.')),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApplicationPlan,
    onSuccess: () => {
      toast.success('پلن اپلیکیشن حذف شد.')
      invalidate()
      setDeleteTarget(null)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف پلن ناموفق بود.')),
  })

  const openCreateDialog = () => {
    setEditingPlan(null)
    setForm(emptyForm)
    setDialogOpen(true)
  }

  const openEditDialog = (plan: ApplicationPlan) => {
    setEditingPlan(plan)
    setForm({
      name: plan.name,
      description: plan.description ?? '',
      priceAmount: plan.price_amount.toString(),
      totalVolumeGb: bytesToGb(plan.total_volume_bytes),
      durationDays: plan.duration_days.toString(),
      maxOnlineUsers: plan.max_online_users.toString(),
      downloadSpeedLimitMbps: plan.download_speed_limit_mbps?.toString() ?? '',
      uploadSpeedLimitMbps: plan.upload_speed_limit_mbps?.toString() ?? '',
    })
    setDialogOpen(true)
  }

  const handleSubmit = () => {
    if (!form.name.trim()) {
      toast.error('نام الزامی است.')
      return
    }

    const downloadLimit = form.downloadSpeedLimitMbps.trim()
      ? Math.round(Number.parseFloat(form.downloadSpeedLimitMbps))
      : null
    const uploadLimit = form.uploadSpeedLimitMbps.trim()
      ? Math.round(Number.parseFloat(form.uploadSpeedLimitMbps))
      : null

    const payload = {
      name: form.name,
      description: form.description || null,
      price_amount: Math.round(Number.parseFloat(form.priceAmount) || 0),
      total_volume_bytes: gbToBytes(form.totalVolumeGb || '0'),
      duration_days: Math.max(1, Math.round(Number.parseFloat(form.durationDays) || 1)),
      max_online_users: Math.max(1, Math.round(Number.parseFloat(form.maxOnlineUsers) || 1)),
      download_speed_limit_mbps: downloadLimit,
      upload_speed_limit_mbps: uploadLimit,
      // clear_speed_limits ensures explicitly emptying a previously-set
      // speed field actually clears it server-side -- a bare `null` on an
      // update request is otherwise indistinguishable from "field omitted,
      // leave unchanged" (see ApplicationPlanService.UpdatePlan's own doc
      // comment on this exact convention).
      clear_speed_limits: editingPlan ? downloadLimit === null && uploadLimit === null : undefined,
    }

    if (editingPlan) {
      updateMutation.mutate({ id: editingPlan.id, ...payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  const toggleActive = (plan: ApplicationPlan) => {
    updateMutation.mutate({ id: plan.id, is_active: !plan.is_active })
  }

  const isSaving = createMutation.isPending || updateMutation.isPending

  return (
    <>
      <Header>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>
      <Main>
        <div className='space-y-4'>
          <div className='flex items-center justify-between'>
            <div>
              <h2 className='text-2xl font-bold tracking-tight'>پلن‌های اپلیکیشن</h2>
              <p className='text-muted-foreground'>
                پلن‌های سطحی (مثلاً برنزی/نقره‌ای/طلایی) برای اپلیکیشن‌ها تعریف
                کنید -- هنگام ساخت اپلیکیشن، انتخاب یک پلن این مقادیر را
                پیش‌پر می‌کند و همچنان قابل تغییر دستی خواهند بود.
              </p>
            </div>
            <Button onClick={openCreateDialog} className='gap-2'>
              <IconPlus className='h-4 w-4' />
              افزودن پلن
            </Button>
          </div>

          <Card>
            <CardContent className='pt-6'>
              {isLoading ? (
                <p className='text-muted-foreground'>در حال بارگذاری...</p>
              ) : plans.length === 0 ? (
                <EmptyState message='هنوز پلنی تعریف نشده است.' />
              ) : (
                <div className='overflow-hidden rounded-lg border'>
                  <Table>
                    <TableHeader>
                      <TableRow className='bg-muted/40'>
                        <TableHead className='text-start'>نام</TableHead>
                        <TableHead className='text-start'>حجم</TableHead>
                        <TableHead className='text-start'>مدت</TableHead>
                        <TableHead className='text-start'>حداکثر کاربر همزمان</TableHead>
                        <TableHead className='text-start'>سرعت</TableHead>
                        <TableHead className='text-start'>قیمت</TableHead>
                        <TableHead className='text-start'>وضعیت</TableHead>
                        <TableHead className='text-start'>عملیات</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {plans.map((plan) => (
                        <TableRow key={plan.id}>
                          <TableCell className='font-medium'>{plan.name}</TableCell>
                          <TableCell>
                            {plan.total_volume_bytes > 0
                              ? `${bytesToGb(plan.total_volume_bytes)} گیگابایت`
                              : 'نامحدود'}
                          </TableCell>
                          <TableCell>
                            {plan.duration_days > 0 ? `${plan.duration_days} روز` : 'نامحدود'}
                          </TableCell>
                          <TableCell>{plan.max_online_users}</TableCell>
                          <TableCell>
                            {plan.download_speed_limit_mbps || plan.upload_speed_limit_mbps
                              ? `${plan.download_speed_limit_mbps ?? '∞'}↓ / ${plan.upload_speed_limit_mbps ?? '∞'}↑ Mbps`
                              : 'نامحدود'}
                          </TableCell>
                          <TableCell className='tabular-nums'>
                            {formatCurrencyFa(plan.price_amount)}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={plan.is_active ? 'default' : 'outline'}
                              className='cursor-pointer'
                              onClick={() => toggleActive(plan)}
                            >
                              {plan.is_active ? 'فعال' : 'غیرفعال'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <div className='flex items-center gap-2'>
                              <Button
                                variant='ghost'
                                size='icon'
                                className='h-8 w-8'
                                onClick={() => openEditDialog(plan)}
                                aria-label='ویرایش پلن'
                              >
                                <IconEdit className='h-4 w-4' />
                              </Button>
                              <Button
                                variant='ghost'
                                size='icon'
                                className='text-destructive h-8 w-8'
                                onClick={() => setDeleteTarget(plan)}
                                aria-label='حذف پلن'
                              >
                                <IconTrash className='h-4 w-4' />
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className='sm:max-w-lg'>
            <DialogHeader>
              <DialogTitle>{editingPlan ? 'ویرایش پلن' : 'افزودن پلن'}</DialogTitle>
              <DialogDescription>
                {editingPlan
                  ? 'این پلن اپلیکیشن را به‌روزرسانی کنید.'
                  : 'یک پلن سطحی جدید برای اپلیکیشن‌ها تعریف کنید.'}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='app-plan-name'>نام</Label>
                <Input
                  id='app-plan-name'
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  placeholder='طلایی'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='app-plan-description'>توضیحات (اختیاری)</Label>
                <Input
                  id='app-plan-description'
                  value={form.description}
                  onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                />
              </div>
              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-volume'>حجم (گیگابایت، ۰=نامحدود)</Label>
                  <Input
                    id='app-plan-volume'
                    type='number'
                    min='0'
                    step='0.1'
                    value={form.totalVolumeGb}
                    onChange={(e) => setForm((f) => ({ ...f, totalVolumeGb: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-duration'>مدت (روز)</Label>
                  <Input
                    id='app-plan-duration'
                    type='number'
                    min='1'
                    value={form.durationDays}
                    onChange={(e) => setForm((f) => ({ ...f, durationDays: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-price'>قیمت (تومان)</Label>
                  <Input
                    id='app-plan-price'
                    type='number'
                    min='0'
                    value={form.priceAmount}
                    onChange={(e) => setForm((f) => ({ ...f, priceAmount: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-max-online'>حداکثر کاربر همزمان</Label>
                  <Input
                    id='app-plan-max-online'
                    type='number'
                    min='1'
                    value={form.maxOnlineUsers}
                    onChange={(e) => setForm((f) => ({ ...f, maxOnlineUsers: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-download-speed'>سرعت دانلود Mbps (خالی=نامحدود)</Label>
                  <Input
                    id='app-plan-download-speed'
                    type='number'
                    min='1'
                    value={form.downloadSpeedLimitMbps}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, downloadSpeedLimitMbps: e.target.value }))
                    }
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='app-plan-upload-speed'>سرعت آپلود Mbps (خالی=نامحدود)</Label>
                  <Input
                    id='app-plan-upload-speed'
                    type='number'
                    min='1'
                    value={form.uploadSpeedLimitMbps}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, uploadSpeedLimitMbps: e.target.value }))
                    }
                  />
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button onClick={handleSubmit} disabled={isSaving}>
                {isSaving ? 'در حال ذخیره...' : 'ذخیره'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <ConfirmDialog
          open={!!deleteTarget}
          onOpenChange={(open) => !open && setDeleteTarget(null)}
          handleConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
          title='حذف پلن؟'
          desc={`پلن "${deleteTarget?.name}" برای همیشه حذف خواهد شد. اپلیکیشن‌هایی که قبلاً روی این پلن ساخته شده‌اند تحت تأثیر قرار نمی‌گیرند.`}
          isLoading={deleteMutation.isPending}
          confirmText='حذف'
          destructive
        />
      </Main>
    </>
  )
}
