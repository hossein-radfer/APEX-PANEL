import { useQuery } from '@tanstack/react-query'
import { IconChartBar } from '@tabler/icons-react'
import { fetchDatabaseSize } from '@/api/system-config.ts'
import { formatBytesFa } from '@/features/reports/lib/format.ts'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

export function DatabaseSizeCard() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['database_size'],
    queryFn: fetchDatabaseSize,
  })

  const topTables = data?.tables.slice(0, 10) ?? []
  const largestBytes = topTables[0]?.bytes_used ?? 0

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconChartBar className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>حجم پایگاه داده</CardTitle>
          <CardDescription>
            حجم کلی فایل پایگاه داده و سهم هر جدول از آن. مخصوص بررسی و
            عیب‌یابی رشد غیرعادی حجم.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : isError ? (
          <p className='text-destructive text-sm'>
            دریافت اطلاعات حجم پایگاه داده ناموفق بود. این قابلیت فقط برای
            نصب‌های SQLite در دسترس است.
          </p>
        ) : (
          <>
            <p className='text-sm'>
              حجم کل فایل:{' '}
              <span className='font-medium'>
                {formatBytesFa(data?.total_bytes)}
              </span>
            </p>

            <div className='space-y-3'>
              {topTables.map((table) => (
                <div key={table.name} className='space-y-1'>
                  <div className='flex items-center justify-between text-sm'>
                    <span className='font-mono text-xs'>{table.name}</span>
                    <span className='text-muted-foreground'>
                      {formatBytesFa(table.bytes_used)}
                    </span>
                  </div>
                  <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                    <div
                      className='bg-primary h-full rounded-full'
                      style={{
                        width: `${
                          largestBytes > 0
                            ? (table.bytes_used / largestBytes) * 100
                            : 0
                        }%`,
                      }}
                    />
                  </div>
                </div>
              ))}
              {topTables.length === 0 && (
                <p className='text-muted-foreground text-sm'>
                  اطلاعاتی برای نمایش وجود ندارد.
                </p>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
