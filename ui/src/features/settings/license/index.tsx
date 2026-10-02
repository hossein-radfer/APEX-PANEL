import { useState } from 'react'
import { toast } from 'sonner'
import {
  IconCalendarTime,
  IconCircleCheck,
  IconCircleX,
  IconClock,
  IconLock,
  IconRefresh,
  IconRocket,
  IconServer,
  IconShieldLock,
  IconTrash,
} from '@tabler/icons-react'
import { useLicenseStatusQuery } from '@/hooks/license/useLicenseStatusQuery.ts'
import { useRevokeLicenseMutation } from '@/hooks/license/useRevokeLicenseMutation.ts'
import { useUpdateCheckQuery } from '@/hooks/license/useUpdateCheckQuery.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import ContentSection from '../components/content-section'

function StatusBadge({
  activated,
  valid,
  inGracePeriod,
}: {
  activated: boolean
  valid: boolean
  inGracePeriod: boolean
}) {
  if (!activated) {
    return (
      <Badge variant='destructive' className='gap-1'>
        <IconCircleX className='size-3.5' />
        فعال‌سازی نشده
      </Badge>
    )
  }
  if (!valid && inGracePeriod) {
    return (
      <Badge variant='outline' className='gap-1 border-amber-500 text-amber-600 dark:text-amber-400'>
        <IconClock className='size-3.5' />
        دوره مهلت
      </Badge>
    )
  }
  if (!valid) {
    return (
      <Badge variant='destructive' className='gap-1'>
        <IconCircleX className='size-3.5' />
        نامعتبر
      </Badge>
    )
  }
  return (
    <Badge className='gap-1 border-emerald-500 bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/15 dark:text-emerald-400'>
      <IconCircleCheck className='size-3.5' />
      فعال
    </Badge>
  )
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='border-border/60 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b py-3 last:border-b-0'>
      <span className='text-muted-foreground text-sm font-medium'>{label}</span>
      <span className='text-foreground text-sm'>{value}</span>
    </div>
  )
}

type PendingAction = 'revoke' | 'replace' | null

export default function SettingsLicense() {
  const { data: license, isLoading, refetch, isRefetching } = useLicenseStatusQuery()
  const revokeMutation = useRevokeLicenseMutation()
  const [pendingAction, setPendingAction] = useState<PendingAction>(null)

  const {
    data: updateInfo,
    isLoading: isUpdateLoading,
    refetch: refetchUpdate,
    isRefetching: isUpdateRefetching,
  } = useUpdateCheckQuery(!!license?.activated && (license?.valid || license?.in_grace_period))

  // Both "Remove License" and "Replace License" call the same backend
  // Revoke endpoint -- there is no separate "replace" API. Revoking clears
  // the stored key, which flips /license/public-status to unactivated, which
  // makes LicenseLockdownGuard render the full activation screen app-wide
  // (see components/license-lockdown-guard.tsx) -- that IS the "enter a new
  // key" flow, so Replace doesn't need its own inline form here.
  const handleConfirm = () => {
    revokeMutation.mutate(undefined, {
      onSuccess: () => {
        toast.success(
          pendingAction === 'replace'
            ? 'لایسنس حذف شد -- برای فعال‌سازی مجدد، یک کلید لایسنس جدید وارد کنید.'
            : 'لایسنس از این نصب حذف شد.'
        )
        setPendingAction(null)
      },
      onError: () => {
        toast.error('حذف لایسنس ناموفق بود. دوباره تلاش کنید.')
      },
    })
  }

  return (
    <ContentSection
      title='لایسنس'
      desc='مشاهده وضعیت لایسنس، پلن و محدودیت‌های این نصب.'
    >
      <div className='space-y-6'>
        <Card>
          <CardHeader className='flex flex-col items-stretch gap-3 pb-2 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3'>
              <IconShieldLock className='h-5 w-5 shrink-0' />
              <div>
                <CardTitle className='text-lg'>وضعیت لایسنس</CardTitle>
                <CardDescription>
                  وضعیت زنده گزارش‌شده توسط سرور مرکزی لایسنس.
                </CardDescription>
              </div>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                className='gap-2'
                disabled={isRefetching}
                onClick={() => refetch()}
              >
                <IconRefresh className={`h-4 w-4 ${isRefetching ? 'animate-spin' : ''}`} />
                بروزرسانی
              </Button>
              {license?.activated && (
                <>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => setPendingAction('replace')}
                  >
                    تغییر لایسنس
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    className='text-destructive hover:text-destructive gap-2'
                    onClick={() => setPendingAction('revoke')}
                  >
                    <IconTrash className='h-4 w-4' />
                    حذف لایسنس
                  </Button>
                </>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {isLoading || !license ? (
              <div className='space-y-3'>
                <Skeleton className='h-6 w-32' />
                <Skeleton className='h-4 w-full' />
                <Skeleton className='h-4 w-full' />
                <Skeleton className='h-4 w-full' />
              </div>
            ) : (
              <div>
                <div className='mb-2'>
                  <StatusBadge
                    activated={license.activated}
                    valid={license.valid}
                    inGracePeriod={license.in_grace_period}
                  />
                </div>
                {license.reason && (
                  <p className='text-muted-foreground mb-3 text-sm'>{license.reason}</p>
                )}
                <InfoRow label='پلن' value={license.plan_name || 'نامشخص'} />
                <InfoRow
                  label='انقضا'
                  value={
                    license.expires_at ? (
                      <span className='flex items-center gap-1.5'>
                        <IconCalendarTime className='size-3.5' />
                        {new Date(license.expires_at).toLocaleDateString()}
                      </span>
                    ) : (
                      'نامشخص'
                    )
                  }
                />
                <InfoRow
                  label='سرورها'
                  value={
                    <span className='flex items-center gap-1.5'>
                      <IconServer className='size-3.5' />
                      {license.server_count} / {license.max_servers || '∞'}
                    </span>
                  }
                />
                <InfoRow
                  label='آخرین بررسی'
                  value={
                    license.last_checked_at
                      ? new Date(license.last_checked_at).toLocaleString()
                      : 'هرگز'
                  }
                />
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-col items-stretch gap-3 pb-2 sm:flex-row sm:items-center sm:justify-between'>
            <div className='flex items-center gap-3'>
              <IconRocket className='h-5 w-5 shrink-0' />
              <div>
                <CardTitle className='text-lg'>بروزرسانی‌ها</CardTitle>
                <CardDescription>
                  بررسی سرور مرکزی لایسنس برای نسخه جدیدتر MWPanel.
                </CardDescription>
              </div>
            </div>
            <Button
              variant='outline'
              size='sm'
              className='gap-2 sm:self-start'
              disabled={isUpdateRefetching || !license?.activated}
              onClick={() => refetchUpdate()}
            >
              <IconRefresh className={`h-4 w-4 ${isUpdateRefetching ? 'animate-spin' : ''}`} />
              بررسی الان
            </Button>
          </CardHeader>
          <CardContent>
            {isUpdateLoading ? (
              <Skeleton className='h-10 w-full' />
            ) : !license?.activated ? (
              <p className='text-muted-foreground text-sm'>
                برای بررسی بروزرسانی‌ها، ابتدا یک لایسنس را در بالا فعال کنید.
              </p>
            ) : updateInfo?.update_available ? (
              <div className='space-y-2'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Badge
                    className={
                      updateInfo.force_update
                        ? 'gap-1'
                        : 'gap-1 border-emerald-500 bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/15 dark:text-emerald-400'
                    }
                    variant={updateInfo.force_update ? 'destructive' : 'outline'}
                  >
                    {updateInfo.force_update ? 'بروزرسانی اجباری' : 'بروزرسانی موجود است'}
                  </Badge>
                  <span className='text-sm font-medium'>v{updateInfo.version}</span>
                </div>
                {updateInfo.change_log && (
                  <p className='text-muted-foreground text-sm whitespace-pre-wrap'>
                    {updateInfo.change_log}
                  </p>
                )}
                <p className='text-muted-foreground text-xs'>
                  در حال حاضر نسخه v{updateInfo.current_version} در حال اجراست. بروزرسانی‌ها
                  روی سرور اعمال می‌شوند، نه از این صفحه -- برای نصب این نسخه با ارائه‌دهنده
                  خود تماس بگیرید یا روند استقرار معمول خود را دنبال کنید.
                </p>
              </div>
            ) : (
              <p className='text-sm'>
                <span className='text-foreground font-medium'>
                  بروز است (v{updateInfo?.current_version ?? '?'})
                </span>
                <span className='text-muted-foreground'> -- نسخه جدیدتری در دسترس نیست.</span>
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-lg'>اتصال به سخت‌افزار</CardTitle>
            <CardDescription>
              این لایسنس به سخت‌افزار این سرور متصل است. انتقال آن به سرور
              جدید نیازمند تماس با پشتیبانی است.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading || !license ? (
              <Skeleton className='h-6 w-48' />
            ) : license.activated && license.valid ? (
              <div className='flex items-center gap-2 text-sm font-medium text-emerald-600 dark:text-emerald-400'>
                <IconLock className='size-4' />
                این سرور به لایسنس متصل است
              </div>
            ) : license.activated && !license.valid ? (
              <div className='text-destructive flex items-center gap-2 text-sm font-medium'>
                <IconCircleX className='size-4' />
                سخت‌افزار نامعتبر است -- برای انتقال این لایسنس به این سرور
                با پشتیبانی تماس بگیرید
              </div>
            ) : (
              <p className='text-muted-foreground text-sm'>
                هنوز به هیچ سخت‌افزاری متصل نشده -- ابتدا یک کلید لایسنس را
                در بالا فعال کنید.
              </p>
            )}
          </CardContent>
        </Card>

        <ConfirmDialog
          open={pendingAction !== null}
          onOpenChange={(open) => !open && setPendingAction(null)}
          title={pendingAction === 'replace' ? 'لایسنس تغییر کند؟' : 'لایسنس حذف شود؟'}
          desc={
            pendingAction === 'replace'
              ? 'این کار کلید لایسنس فعلی را از این سرور جدا می‌کند تا بتوانید لایسنس دیگری فعال کنید. این نصب تا وارد کردن یک کلید جدید قفل خواهد بود.'
              : 'این کار کلید لایسنس فعلی را از این سرور جدا می‌کند. این نصب قفل شده و تا وارد کردن دوباره یک کلید لایسنس، صفحه فعال‌سازی را نمایش خواهد داد.'
          }
          destructive
          confirmText={pendingAction === 'replace' ? 'تغییر لایسنس' : 'حذف لایسنس'}
          isLoading={revokeMutation.isPending}
          handleConfirm={handleConfirm}
        />
      </div>
    </ContentSection>
  )
}
