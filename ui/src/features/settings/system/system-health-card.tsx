import { useQuery } from '@tanstack/react-query'
import {
  IconActivity,
  IconCheck,
  IconCpu,
  IconDatabase,
  IconServer,
  IconX,
} from '@tabler/icons-react'
import { fetchSystemHealth } from '@/api/system-config.ts'
import { formatBytesFa } from '@/features/reports/lib/format.ts'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

// Live health tiles refresh every 10s -- fast enough to feel "live" for a
// glance-and-leave admin page, not so fast it hammers the endpoint (which
// itself does a synchronous /proc read + a disk syscall on every call).
const HEALTH_REFRESH_INTERVAL_MS = 10_000

function UsageBar({
  label,
  usedBytes,
  totalBytes,
  icon: Icon,
}: {
  label: string
  usedBytes: number
  totalBytes: number
  icon: React.ComponentType<{ className?: string }>
}) {
  const percent = totalBytes > 0 ? (usedBytes / totalBytes) * 100 : 0
  return (
    <div className='space-y-1'>
      <div className='flex items-center justify-between text-sm'>
        <span className='flex items-center gap-1.5'>
          <Icon className='h-4 w-4' />
          {label}
        </span>
        <span className='text-muted-foreground'>
          {formatBytesFa(usedBytes)} از {formatBytesFa(totalBytes)}
        </span>
      </div>
      <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
        <div
          className={`h-full rounded-full ${
            percent >= 90
              ? 'bg-destructive'
              : percent >= 75
                ? 'bg-amber-500'
                : 'bg-primary'
          }`}
          style={{ width: `${Math.min(100, percent)}%` }}
        />
      </div>
    </div>
  )
}

export function SystemHealthCard() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['system_health'],
    queryFn: fetchSystemHealth,
    refetchInterval: HEALTH_REFRESH_INTERVAL_MS,
  })

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconActivity className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>وضعیت سلامت پنل</CardTitle>
          <CardDescription>
            مصرف زنده‌ی منابع سرور و وضعیت آخرین بکاپ خودکار.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : isError || !data ? (
          <p className='text-destructive text-sm'>
            دریافت اطلاعات سلامت پنل ناموفق بود. این قابلیت فقط روی سرورهای
            لینوکسی در دسترس است.
          </p>
        ) : (
          <>
            <div className='space-y-1'>
              <div className='flex items-center justify-between text-sm'>
                <span className='flex items-center gap-1.5'>
                  <IconCpu className='h-4 w-4' />
                  پردازنده (CPU)
                </span>
                <span className='text-muted-foreground'>
                  {data.cpu_percent < 0
                    ? 'در حال محاسبه...'
                    : `${data.cpu_percent.toFixed(1)}٪`}
                </span>
              </div>
              {data.cpu_percent >= 0 && (
                <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                  <div
                    className={`h-full rounded-full ${
                      data.cpu_percent >= 90
                        ? 'bg-destructive'
                        : data.cpu_percent >= 75
                          ? 'bg-amber-500'
                          : 'bg-primary'
                    }`}
                    style={{ width: `${Math.min(100, data.cpu_percent)}%` }}
                  />
                </div>
              )}
            </div>

            <UsageBar
              label='حافظه (RAM)'
              usedBytes={data.memory_used_bytes}
              totalBytes={data.memory_total_bytes}
              icon={IconServer}
            />

            <UsageBar
              label='دیسک (مسیر داده)'
              usedBytes={data.disk_used_bytes}
              totalBytes={data.disk_total_bytes}
              icon={IconDatabase}
            />

            <div className='flex items-center justify-between rounded-lg border p-3 text-sm'>
              <span>آخرین بکاپ خودکار</span>
              {!data.auto_backup_enabled ? (
                <span className='text-muted-foreground'>غیرفعال</span>
              ) : data.backup_is_up_to_date ? (
                <span className='flex items-center gap-1 text-emerald-600'>
                  <IconCheck className='h-4 w-4' />
                  امروز ارسال شد
                </span>
              ) : (
                <span className='text-destructive flex items-center gap-1'>
                  <IconX className='h-4 w-4' />
                  {data.last_backup_sent_date
                    ? `آخرین بار: ${data.last_backup_sent_date}`
                    : 'هرگز ارسال نشده'}
                </span>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
