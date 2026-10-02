import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconEdit, IconPlus, IconTrash } from '@tabler/icons-react'
import {
  createDNSPlan,
  deleteDNSPlan,
  fetchDNSPlans,
  updateDNSPlan,
} from '@/api/dns-plan.ts'
import { DNSPlan } from '@/schema/dns-plan.ts'
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

// Admin-facing CRUD for phase 4-3's DNS tier catalog (برنزی/نقره‌ای/طلایی)
// -- see the backend's model.DNSPlan doc comment for why this is a global
// catalog, structurally mirroring TrafficPackagesPage's admin view rather
// than doctor-dns's own remote per-panel "template" picker (a completely
// separate concept, already surfaced elsewhere in the DNS account form).

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
  speedKbps: string
  durationDays: string
  maxConcurrentIps: string
  dailyIpRegistrationLimit: string
}

const emptyForm: PlanFormState = {
  name: '',
  description: '',
  priceAmount: '',
  totalVolumeGb: '',
  speedKbps: '0',
  durationDays: '30',
  maxConcurrentIps: '1',
  dailyIpRegistrationLimit: '0',
}

export default function DNSPlansPage() {
  const queryClient = useQueryClient()
  const { data: plans = [], isLoading } = useQuery({
    queryKey: ['dns_plans'],
    queryFn: fetchDNSPlans,
  })

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingPlan, setEditingPlan] = useState<DNSPlan | null>(null)
  const [form, setForm] = useState<PlanFormState>(emptyForm)
  const [deleteTarget, setDeleteTarget] = useState<DNSPlan | null>(null)

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['dns_plans'] })

  const createMutation = useMutation({
    mutationFn: createDNSPlan,
    onSuccess: () => {
      toast.success('پلن DNS ایجاد شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ایجاد پلن ناموفق بود.')),
  })

  const updateMutation = useMutation({
    mutationFn: updateDNSPlan,
    onSuccess: () => {
      toast.success('پلن DNS به‌روزرسانی شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی پلن ناموفق بود.')),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDNSPlan,
    onSuccess: () => {
      toast.success('پلن DNS حذف شد.')
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

  const openEditDialog = (plan: DNSPlan) => {
    setEditingPlan(plan)
    setForm({
      name: plan.name,
      description: plan.description ?? '',
      priceAmount: plan.price_amount.toString(),
      totalVolumeGb: bytesToGb(plan.total_volume_bytes),
      speedKbps: plan.speed_kbps.toString(),
      durationDays: plan.duration_days.toString(),
      maxConcurrentIps: plan.max_concurrent_ips.toString(),
      dailyIpRegistrationLimit: plan.daily_ip_registration_limit.toString(),
    })
    setDialogOpen(true)
  }

  const handleSubmit = () => {
    if (!form.name.trim()) {
      toast.error('نام الزامی است.')
      return
    }

    const payload = {
      name: form.name,
      description: form.description || null,
      price_amount: Math.round(Number.parseFloat(form.priceAmount) || 0),
      total_volume_bytes: gbToBytes(form.totalVolumeGb || '0'),
      speed_kbps: Math.round(Number.parseFloat(form.speedKbps) || 0),
      duration_days: Math.round(Number.parseFloat(form.durationDays) || 0),
      max_concurrent_ips: Math.max(1, Math.round(Number.parseFloat(form.maxConcurrentIps) || 1)),
      daily_ip_registration_limit: Math.round(
        Number.parseFloat(form.dailyIpRegistrationLimit) || 0
      ),
    }

    if (editingPlan) {
      updateMutation.mutate({ id: editingPlan.id, ...payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  const toggleActive = (plan: DNSPlan) => {
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
              <h2 className='text-2xl font-bold tracking-tight'>پلن‌های DNS</h2>
              <p className='text-muted-foreground'>
                پلن‌های سطحی (مثلاً برنزی/نقره‌ای/طلایی) برای اکانت‌های Smart DNS
                تعریف کنید -- هنگام ساخت یا ویرایش اکانت، انتخاب یک پلن این
                مقادیر را پیش‌پر می‌کند و همچنان قابل تغییر دستی خواهند بود.
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
                        <TableHead className='text-start'>سرعت</TableHead>
                        <TableHead className='text-start'>مدت</TableHead>
                        <TableHead className='text-start'>حداکثر IP</TableHead>
                        <TableHead className='text-start'>ثبت روزانه</TableHead>
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
                            {plan.speed_kbps > 0 ? `${plan.speed_kbps} Kbps` : 'نامحدود'}
                          </TableCell>
                          <TableCell>
                            {plan.duration_days > 0 ? `${plan.duration_days} روز` : 'نامحدود'}
                          </TableCell>
                          <TableCell>{plan.max_concurrent_ips}</TableCell>
                          <TableCell>
                            {plan.daily_ip_registration_limit > 0
                              ? plan.daily_ip_registration_limit
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
                  ? 'این پلن DNS را به‌روزرسانی کنید.'
                  : 'یک پلن سطحی جدید برای اکانت‌های Smart DNS تعریف کنید.'}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='plan-name'>نام</Label>
                <Input
                  id='plan-name'
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  placeholder='طلایی'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='plan-description'>توضیحات (اختیاری)</Label>
                <Input
                  id='plan-description'
                  value={form.description}
                  onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                />
              </div>
              <div className='grid grid-cols-2 gap-4'>
                <div className='space-y-2'>
                  <Label htmlFor='plan-volume'>حجم (گیگابایت، ۰=نامحدود)</Label>
                  <Input
                    id='plan-volume'
                    type='number'
                    min='0'
                    step='0.1'
                    value={form.totalVolumeGb}
                    onChange={(e) => setForm((f) => ({ ...f, totalVolumeGb: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='plan-speed'>سرعت (Kbps، ۰=نامحدود)</Label>
                  <Input
                    id='plan-speed'
                    type='number'
                    min='0'
                    value={form.speedKbps}
                    onChange={(e) => setForm((f) => ({ ...f, speedKbps: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='plan-duration'>مدت (روز، ۰=نامحدود)</Label>
                  <Input
                    id='plan-duration'
                    type='number'
                    min='0'
                    value={form.durationDays}
                    onChange={(e) => setForm((f) => ({ ...f, durationDays: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='plan-price'>قیمت (تومان)</Label>
                  <Input
                    id='plan-price'
                    type='number'
                    min='0'
                    value={form.priceAmount}
                    onChange={(e) => setForm((f) => ({ ...f, priceAmount: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='plan-max-ips'>حداکثر IP هم‌زمان</Label>
                  <Input
                    id='plan-max-ips'
                    type='number'
                    min='1'
                    value={form.maxConcurrentIps}
                    onChange={(e) => setForm((f) => ({ ...f, maxConcurrentIps: e.target.value }))}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='plan-daily-limit'>ثبت IP روزانه (۰=نامحدود)</Label>
                  <Input
                    id='plan-daily-limit'
                    type='number'
                    min='0'
                    value={form.dailyIpRegistrationLimit}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, dailyIpRegistrationLimit: e.target.value }))
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
          desc={`پلن "${deleteTarget?.name}" برای همیشه حذف خواهد شد. اکانت‌هایی که قبلاً روی این پلن ساخته شده‌اند تحت تأثیر قرار نمی‌گیرند.`}
          isLoading={deleteMutation.isPending}
          confirmText='حذف'
          destructive
        />
      </Main>
    </>
  )
}
