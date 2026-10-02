import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconTrash } from '@tabler/icons-react'
import {
  clearConnectionHistory,
  clearEtherTraffic,
  runRetentionCleanup,
} from '@/api/security.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

const AGE_OPTIONS = [
  { label: 'یک روز پیش', days: 1 },
  { label: 'سه روز پیش', days: 3 },
  { label: 'هفت روز پیش', days: 7 },
  { label: 'یک ماه پیش', days: 30 },
]

export function RetentionCard() {
  const queryClient = useQueryClient()
  const [lastResult, setLastResult] = useState<string | null>(null)

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ['security_identities'] })
    queryClient.invalidateQueries({ queryKey: ['security_ether_traffic'] })
  }

  const cleanupMutation = useMutation({
    mutationFn: runRetentionCleanup,
    onSuccess: (deletedCount, variables) => {
      setLastResult(`${deletedCount.toLocaleString('fa-IR')} ردیف حذف شد.`)
      toast.success('پاکسازی با موفقیت انجام شد.')
      invalidateAll()
      void variables
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'پاکسازی ناموفق بود.')),
  })

  const clearHistoryMutation = useMutation({
    mutationFn: clearConnectionHistory,
    onSuccess: () => {
      toast.success('تاریخچه‌ی اتصال کاربران به‌طور کامل پاک شد.')
      invalidateAll()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'پاکسازی ناموفق بود.')),
  })

  const clearTrafficMutation = useMutation({
    mutationFn: clearEtherTraffic,
    onSuccess: () => {
      toast.success('تاریخچه‌ی ترافیک اتریک به‌طور کامل پاک شد.')
      invalidateAll()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'پاکسازی ناموفق بود.')),
  })

  const isMutating =
    cleanupMutation.isPending || clearHistoryMutation.isPending || clearTrafficMutation.isPending

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconTrash className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>پاکسازی داده‌ها</CardTitle>
          <CardDescription>
            برای جلوگیری از سنگین شدن دیتابیس پنل، تاریخچه‌ی اتصال و ترافیک را
            به‌صورت دوره‌ای پاکسازی کنید.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-6'>
        <div className='space-y-2'>
          <p className='text-sm font-medium'>حذف تاریخچه‌ی اتصال بر اساس بازه</p>
          <div className='flex flex-wrap gap-2'>
            {AGE_OPTIONS.map((option) => (
              <Button
                key={option.days}
                variant='outline'
                size='sm'
                disabled={isMutating}
                onClick={() =>
                  cleanupMutation.mutate({
                    target: 'connection_log',
                    older_than_days: option.days,
                  })
                }
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>

        <div className='space-y-2'>
          <p className='text-sm font-medium'>حذف تاریخچه‌ی ترافیک اتریک بر اساس بازه</p>
          <div className='flex flex-wrap gap-2'>
            {AGE_OPTIONS.map((option) => (
              <Button
                key={option.days}
                variant='outline'
                size='sm'
                disabled={isMutating}
                onClick={() =>
                  cleanupMutation.mutate({
                    target: 'ether_traffic',
                    older_than_days: option.days,
                  })
                }
              >
                {option.label}
              </Button>
            ))}
          </div>
        </div>

        <div className='space-y-2'>
          <p className='text-sm font-medium'>
            کاربرانی که حجم یا زمانشان تمام شده یا حذف شده‌اند
          </p>
          <Button
            variant='outline'
            size='sm'
            disabled={isMutating}
            onClick={() =>
              cleanupMutation.mutate({
                target: 'connection_log',
                inactive_users_only: true,
              })
            }
          >
            پاکسازی کاربران غیرفعال
          </Button>
        </div>

        {lastResult && (
          <p className='text-muted-foreground text-sm'>{lastResult}</p>
        )}

        <div className='flex flex-wrap gap-2 border-t pt-4'>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant='destructive' size='sm' disabled={isMutating}>
                پاکسازی کامل تاریخچه‌ی اتصال
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>پاکسازی کامل تاریخچه‌ی اتصال؟</AlertDialogTitle>
                <AlertDialogDescription>
                  این عمل تمام تاریخچه‌ی اتصال IP کاربران را برای همیشه حذف
                  می‌کند و قابل بازگشت نیست.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>انصراف</AlertDialogCancel>
                <AlertDialogAction onClick={() => clearHistoryMutation.mutate()}>
                  حذف کامل
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant='destructive' size='sm' disabled={isMutating}>
                پاکسازی کامل ترافیک اتریک
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>پاکسازی کامل ترافیک اتریک؟</AlertDialogTitle>
                <AlertDialogDescription>
                  این عمل تمام تاریخچه‌ی نمونه‌های ترافیک اتریک را برای همیشه
                  حذف می‌کند و قابل بازگشت نیست.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>انصراف</AlertDialogCancel>
                <AlertDialogAction onClick={() => clearTrafficMutation.mutate()}>
                  حذف کامل
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </CardContent>
    </Card>
  )
}
