import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  adminDisableFaoxima,
  adminEnableFaoxima,
  adminResetFaoximaPeriod,
  fetchFaoximaAdminInstances,
} from '@/api/faoxima.ts'
import { FaoximaAdminInstance } from '@/schema/faoxima.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const statusLabels: Record<
  string,
  { label: string; variant: 'default' | 'secondary' | 'destructive' }
> = {
  PROVISIONING: { label: 'در حال راه‌اندازی', variant: 'secondary' },
  ENABLED: { label: 'فعال', variant: 'default' },
  DISABLED: { label: 'غیرفعال', variant: 'secondary' },
  ERROR: { label: 'خطا', variant: 'destructive' },
}

function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  return new Date(value).toLocaleDateString('fa-IR')
}

// EnableDialog collects the billing period (days) before calling the
// admin's own "فعال‌سازی" action -- the admin's explicit request: a
// per-reseller period tied to enable/disable, e.g. "دوره ۳۰ روزه نماینده
// پولشو پرداخت کرده و بعد ۳۰ روز غیرفعال بشه تا من ریست کنم."
function EnableDialog({
  instance,
  open,
  onClose,
}: {
  instance: FaoximaAdminInstance | null
  open: boolean
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [periodDays, setPeriodDays] = useState('30')

  const enableMutation = useMutation({
    mutationFn: adminEnableFaoxima,
    onSuccess: () => {
      toast.success('ربات فعال شد.')
      queryClient.invalidateQueries({ queryKey: ['faoxima_admin_instances'] })
      onClose()
    },
    onError: (err) =>
      toast.error(getApiErrorMessage(err, 'فعال‌سازی ناموفق بود.')),
  })

  if (!instance) return null

  const handleConfirm = () => {
    const days = Number(periodDays)
    enableMutation.mutate({
      resellerId: instance.reseller_id,
      period_days: Number.isFinite(days) && days > 0 ? days : 0,
    })
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>فعال‌سازی ربات ایکس {instance.reseller_name}</DialogTitle>
        </DialogHeader>
        <div className='space-y-2 py-2'>
          <Label htmlFor='period-days'>دوره‌ی صورتحساب (روز)</Label>
          <Input
            id='period-days'
            type='number'
            min='0'
            value={periodDays}
            onChange={(e) => setPeriodDays(e.target.value)}
            placeholder='مثلاً ۳۰'
          />
          <p className='text-muted-foreground text-xs'>
            بعد از این تعداد روز، ربات به‌صورت خودکار غیرفعال می‌شود تا
            دوباره «ریست دوره» بزنید. عدد صفر یا خالی یعنی بدون محدودیت
            زمانی.
          </p>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={onClose}>
            انصراف
          </Button>
          <Button onClick={handleConfirm} disabled={enableMutation.isPending}>
            {enableMutation.isPending ? 'در حال فعال‌سازی...' : 'فعال‌سازی'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function InstancesTable() {
  const queryClient = useQueryClient()
  const { data: instances = [], isLoading } = useQuery({
    queryKey: ['faoxima_admin_instances'],
    queryFn: fetchFaoximaAdminInstances,
  })
  const [enableTarget, setEnableTarget] = useState<FaoximaAdminInstance | null>(
    null
  )

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['faoxima_admin_instances'] })

  const disableMutation = useMutation({
    mutationFn: adminDisableFaoxima,
    onSuccess: () => {
      toast.success('ربات غیرفعال شد.')
      invalidate()
    },
    onError: (err) =>
      toast.error(getApiErrorMessage(err, 'غیرفعال‌سازی ناموفق بود.')),
  })

  const resetPeriodMutation = useMutation({
    mutationFn: adminResetFaoximaPeriod,
    onSuccess: () => {
      toast.success('دوره‌ی صورتحساب ریست و ربات فعال شد.')
      invalidate()
    },
    onError: (err) =>
      toast.error(getApiErrorMessage(err, 'ریست دوره ناموفق بود.')),
  })

  if (isLoading) {
    return <Skeleton className='h-64 w-full rounded-lg' />
  }

  if (instances.length === 0) {
    return (
      <EmptyState message='هنوز هیچ نماینده‌ای ربات ایکس راه‌اندازی نکرده است.' />
    )
  }

  return (
    <>
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>نماینده</TableHead>
              <TableHead>وضعیت</TableHead>
              <TableHead>دوره‌ی صورتحساب</TableHead>
              <TableHead>پایان دوره</TableHead>
              <TableHead>عملیات</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {instances.map((instance) => {
              const status = statusLabels[instance.status] ?? statusLabels.ERROR
              return (
                <TableRow key={instance.reseller_id}>
                  <TableCell className='font-medium'>
                    {instance.reseller_name}
                  </TableCell>
                  <TableCell>
                    <Badge variant={status.variant}>{status.label}</Badge>
                  </TableCell>
                  <TableCell className='text-muted-foreground text-sm'>
                    {instance.billing_period_days
                      ? `${instance.billing_period_days} روز`
                      : 'نامحدود'}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-sm'>
                    {formatDate(instance.period_expires_at)}
                  </TableCell>
                  <TableCell>
                    <div className='flex flex-wrap gap-2'>
                      {instance.status === 'ENABLED' ? (
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={disableMutation.isPending}
                          onClick={() =>
                            disableMutation.mutate(instance.reseller_id)
                          }
                        >
                          غیرفعال‌سازی
                        </Button>
                      ) : (
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => setEnableTarget(instance)}
                        >
                          فعال‌سازی
                        </Button>
                      )}
                      {instance.billing_period_days != null && (
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={resetPeriodMutation.isPending}
                          onClick={() =>
                            resetPeriodMutation.mutate(instance.reseller_id)
                          }
                        >
                          ریست دوره
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>

      <EnableDialog
        instance={enableTarget}
        open={enableTarget !== null}
        onClose={() => setEnableTarget(null)}
      />
    </>
  )
}

export default function FaoximaManagement() {
  return (
    <>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-4'>
          <h2 className='text-2xl font-bold tracking-tight'>
            مدیریت ربات ایکس نمایندگان
          </h2>
          <p className='text-muted-foreground'>
            هر ربات ایکس نماینده منابع واقعی سرور (ترافیک و حافظه)
            مصرف می‌کند. از این بخش می‌توانید ربات هر نماینده را فعال یا
            غیرفعال کنید و برای فعال‌سازی، یک دوره‌ی صورتحساب (مثلاً ۳۰
            روزه) تعیین کنید -- پس از پایان دوره، ربات به‌صورت خودکار
            غیرفعال می‌شود تا شما پس از دریافت پرداخت بعدی، «ریست دوره»
            را بزنید.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className='text-lg'>فهرست ربات‌های ایکس نمایندگان</CardTitle>
          </CardHeader>
          <CardContent>
            <InstancesTable />
          </CardContent>
        </Card>
      </Main>
    </>
  )
}
