import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import {
  IconPackage,
  IconPlus,
  IconShoppingCart,
  IconHistory,
} from '@tabler/icons-react'
import {
  createV2RayTrafficPackage,
  deleteV2RayTrafficPackage,
  fetchActiveV2RayTrafficPackages,
  fetchAllV2RayTrafficPackages,
  fetchMyV2RayPackagePurchases,
  purchaseV2RayTrafficPackage,
  updateV2RayTrafficPackage,
} from '@/api/v2ray-traffic-packages.ts'
import { V2RayTrafficPackage } from '@/schema/v2ray-traffic-package.ts'
import { TrafficPackageTable } from '@/features/billing/components/traffic-package-table.tsx'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { formatCurrencyFa, formatDateTimeFa } from '@/features/reports/lib/format.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { ThemeSwitch } from '@/components/theme-switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/confirm-dialog'

// Mirrors features/billing/user-manager-traffic-packages-page.tsx exactly,
// but for the separate V2Ray traffic-package pool -- resellers with V2Ray
// access but no WireGuard/User Manager quota need their own place to buy
// extra traffic against V2RayQuotaBytes. Whole-Toman amounts throughout --
// see traffic-packages-page.tsx's own comment for the rationale.

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

interface PackageFormState {
  name: string
  description: string
  trafficGb: string
  priceAmount: string
}

const emptyForm: PackageFormState = {
  name: '',
  description: '',
  trafficGb: '',
  priceAmount: '',
}

function AdminV2RayTrafficPackagesView() {
  const queryClient = useQueryClient()
  const { data: packages = [], isLoading } = useQuery({
    queryKey: ['v2ray_traffic_packages_all'],
    queryFn: fetchAllV2RayTrafficPackages,
  })

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingPackage, setEditingPackage] =
    useState<V2RayTrafficPackage | null>(null)
  const [form, setForm] = useState<PackageFormState>(emptyForm)
  const [deleteTarget, setDeleteTarget] = useState<V2RayTrafficPackage | null>(
    null
  )

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['v2ray_traffic_packages_all'] })
    queryClient.invalidateQueries({
      queryKey: ['v2ray_traffic_packages_active'],
    })
  }

  const createMutation = useMutation({
    mutationFn: createV2RayTrafficPackage,
    onSuccess: () => {
      toast.success('بسته ترافیک V2Ray ایجاد شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'ایجاد بسته ناموفق بود.')),
  })

  const updateMutation = useMutation({
    mutationFn: updateV2RayTrafficPackage,
    onSuccess: () => {
      toast.success('بسته ترافیک V2Ray به‌روزرسانی شد.')
      invalidate()
      setDialogOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'به‌روزرسانی بسته ناموفق بود.')),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteV2RayTrafficPackage,
    onSuccess: () => {
      toast.success('بسته ترافیک V2Ray حذف شد.')
      invalidate()
      setDeleteTarget(null)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف بسته ناموفق بود.')),
  })

  const openCreateDialog = () => {
    setEditingPackage(null)
    setForm(emptyForm)
    setDialogOpen(true)
  }

  const openEditDialog = (pkg: V2RayTrafficPackage) => {
    setEditingPackage(pkg)
    setForm({
      name: pkg.name,
      description: pkg.description ?? '',
      trafficGb: bytesToGb(pkg.traffic_bytes),
      priceAmount: pkg.price_amount.toString(),
    })
    setDialogOpen(true)
  }

  const handleSubmit = () => {
    const trafficBytes = gbToBytes(form.trafficGb)
    const priceAmount = Math.round(Number.parseFloat(form.priceAmount))

    if (!form.name.trim()) {
      toast.error('نام الزامی است.')
      return
    }
    if (!Number.isFinite(trafficBytes) || trafficBytes <= 0) {
      toast.error('حجم ترافیک باید عددی مثبت (گیگابایت) باشد.')
      return
    }
    if (!Number.isFinite(priceAmount) || priceAmount < 0) {
      toast.error('قیمت باید عددی غیرمنفی باشد.')
      return
    }

    if (editingPackage) {
      updateMutation.mutate({
        id: editingPackage.id,
        name: form.name,
        description: form.description || null,
        traffic_bytes: trafficBytes,
        price_amount: priceAmount,
      })
    } else {
      createMutation.mutate({
        name: form.name,
        description: form.description || null,
        traffic_bytes: trafficBytes,
        price_amount: priceAmount,
      })
    }
  }

  const toggleActive = (pkg: V2RayTrafficPackage) => {
    updateMutation.mutate({ id: pkg.id, is_active: !pkg.is_active })
  }

  const isSaving = createMutation.isPending || updateMutation.isPending

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>
            بسته‌های ترافیک V2Ray
          </h2>
          <p className='text-muted-foreground'>
            بسته‌های ترافیک اضافه‌ای که نمایندگان می‌توانند برای سهمیه
            V2Ray خود خریداری کنند را تعریف کنید.
          </p>
        </div>
        <Button onClick={openCreateDialog} className='gap-2'>
          <IconPlus className='h-4 w-4' />
          افزودن بسته
        </Button>
      </div>

      <Card>
        <CardContent className='pt-6'>
          <TrafficPackageTable
            packages={packages}
            isLoading={isLoading}
            bytesToGb={(bytes) => `${bytesToGb(bytes)} گیگابایت`}
            onToggleActive={toggleActive}
            onEdit={openEditDialog}
            onDelete={setDeleteTarget}
          />
        </CardContent>
      </Card>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{editingPackage ? 'ویرایش بسته' : 'افزودن بسته'}</DialogTitle>
            <DialogDescription>
              {editingPackage
                ? 'این بسته ترافیک را به‌روزرسانی کنید.'
                : 'یک بسته ترافیک اضافه جدید برای خرید نمایندگان تعریف کنید.'}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='v2ray-pkg-name'>نام</Label>
              <Input
                id='v2ray-pkg-name'
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                placeholder='۱۰ گیگابایت افزایشی'
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='v2ray-pkg-description'>توضیحات (اختیاری)</Label>
              <Input
                id='v2ray-pkg-description'
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              />
            </div>
            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-2'>
                <Label htmlFor='v2ray-pkg-traffic'>ترافیک (گیگابایت)</Label>
                <Input
                  id='v2ray-pkg-traffic'
                  type='number'
                  min='0.01'
                  step='0.01'
                  value={form.trafficGb}
                  onChange={(e) => setForm((f) => ({ ...f, trafficGb: e.target.value }))}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='v2ray-pkg-price'>قیمت (تومان)</Label>
                <Input
                  id='v2ray-pkg-price'
                  type='number'
                  min='0'
                  step='1'
                  value={form.priceAmount}
                  onChange={(e) => setForm((f) => ({ ...f, priceAmount: e.target.value }))}
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
        title='حذف بسته؟'
        desc={`بسته "${deleteTarget?.name}" برای همیشه حذف خواهد شد. رسیدهای خرید قبلی تحت تأثیر قرار نمی‌گیرند.`}
        isLoading={deleteMutation.isPending}
        confirmText='حذف'
        destructive
      />
    </div>
  )
}

function ResellerV2RayTrafficPackagesView() {
  const queryClient = useQueryClient()
  const { data: packages = [], isLoading } = useQuery({
    queryKey: ['v2ray_traffic_packages_active'],
    queryFn: fetchActiveV2RayTrafficPackages,
  })
  const { data: purchases = [], isLoading: isPurchasesLoading } = useQuery({
    queryKey: ['my_v2ray_package_purchases'],
    queryFn: fetchMyV2RayPackagePurchases,
  })

  const purchaseMutation = useMutation({
    mutationFn: purchaseV2RayTrafficPackage,
    onSuccess: (purchase) => {
      toast.success(
        `بسته «${purchase.traffic_package_name}» خریداری شد: ${bytesToGb(purchase.traffic_bytes)} گیگابایت به سهمیه V2Ray شما افزوده شد.`
      )
      queryClient.invalidateQueries({ queryKey: ['my_v2ray_package_purchases'] })
      queryClient.invalidateQueries({ queryKey: ['reseller'] })
      queryClient.invalidateQueries({ queryKey: ['wallet_balance'] })
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'خرید بسته ناموفق بود.')),
  })

  return (
    <div className='space-y-4'>
      <div>
        <h2 className='text-2xl font-bold tracking-tight'>
          خرید ترافیک V2Ray
        </h2>
        <p className='text-muted-foreground'>
          سهمیه V2Ray شما تمام شده؟ با استفاده از موجودی کیف پول خود یک
          بسته ترافیک اضافه بخرید.
        </p>
      </div>

      {isLoading ? (
        <p className='text-muted-foreground'>در حال بارگذاری...</p>
      ) : packages.length === 0 ? (
        <Card>
          <CardContent className='pt-6'>
            <p className='text-muted-foreground'>در حال حاضر بسته ترافیکی موجود نیست.</p>
          </CardContent>
        </Card>
      ) : (
        <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-3'>
          {packages.map((pkg) => (
            <Card key={pkg.id}>
              <CardHeader className='flex flex-row items-center gap-3 pb-2'>
                <IconPackage className='h-5 w-5' />
                <CardTitle className='text-lg'>{pkg.name}</CardTitle>
              </CardHeader>
              <CardContent className='space-y-3'>
                {pkg.description && (
                  <p className='text-muted-foreground text-sm'>{pkg.description}</p>
                )}
                <p className='text-2xl font-bold'>{bytesToGb(pkg.traffic_bytes)} گیگابایت</p>
                <p className='text-muted-foreground text-sm tabular-nums'>
                  {formatCurrencyFa(pkg.price_amount)}
                </p>
                <Button
                  className='w-full gap-2'
                  onClick={() => purchaseMutation.mutate(pkg.id)}
                  disabled={purchaseMutation.isPending}
                >
                  <IconShoppingCart className='h-4 w-4' />
                  {purchaseMutation.isPending ? 'در حال پردازش...' : 'خرید'}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Card>
        <CardHeader className='flex flex-row items-center gap-3 pb-2'>
          <IconHistory className='h-5 w-5' />
          <CardTitle>تاریخچه خرید</CardTitle>
        </CardHeader>
        <CardContent>
          {isPurchasesLoading ? (
            <p className='text-muted-foreground'>در حال بارگذاری...</p>
          ) : purchases.length === 0 ? (
            <p className='text-muted-foreground'>هنوز خریدی ثبت نشده است.</p>
          ) : (
            <div className='space-y-2'>
              {purchases.map((p) => (
                <div
                  key={p.id}
                  className='flex items-center justify-between rounded-lg border px-4 py-3'
                >
                  <div>
                    <p className='text-sm font-medium'>{p.traffic_package_name}</p>
                    <p className='text-muted-foreground text-xs'>
                      {formatDateTimeFa(new Date(p.created_at * 1000).toISOString())}
                    </p>
                  </div>
                  <div className='text-right'>
                    <p className='font-bold'>+{bytesToGb(p.traffic_bytes)} گیگابایت</p>
                    <p className='text-muted-foreground text-xs tabular-nums'>
                      {formatCurrencyFa(p.price_amount)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default function V2RayTrafficPackagesPage() {
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'

  return (
    <>
      <Header>
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>
      <Main>
        {isReseller ? (
          <ResellerV2RayTrafficPackagesView />
        ) : (
          <AdminV2RayTrafficPackagesView />
        )}
      </Main>
    </>
  )
}
