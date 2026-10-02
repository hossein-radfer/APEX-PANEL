import { useQuery } from '@tanstack/react-query'
import { IconActivity } from '@tabler/icons-react'
import { fetchEtherTraffic } from '@/api/security.ts'
import { formatBytesFa, formatDateTimeFa } from '@/features/reports/lib/format.ts'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// Polled every 10s by the panel backend (see
// SecurityEtherTorchService.Poll) -- this card re-fetches on the same
// cadence so the table stays roughly in sync with the latest sample tick,
// per the admin's own "به صورت لحظه‌ای" (moment-to-moment) requirement.
const REFRESH_INTERVAL_MS = 10_000

export function EtherTrafficCard() {
  const { data, isLoading, dataUpdatedAt } = useQuery({
    queryKey: ['security_ether_traffic'],
    queryFn: fetchEtherTraffic,
    refetchInterval: REFRESH_INTERVAL_MS,
  })

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='space-y-0'>
        <div className='flex items-center gap-2'>
          <IconActivity className='h-5 w-5' />
          <CardTitle className='text-lg'>ترافیک کلی (اتریک)</CardTitle>
        </div>
        <CardDescription>
          جریان‌های شبکه‌ی لحظه‌ای روی اینترفیس اصلی -- هر ۱۰ ثانیه به‌روزرسانی
          می‌شود
          {dataUpdatedAt > 0 && (
            <> (آخرین به‌روزرسانی: {formatDateTimeFa(new Date(dataUpdatedAt).toISOString())})</>
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-64 w-full rounded-lg' />
        ) : !data || data.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            در حال حاضر جریان فعالی ثبت نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-start'>آی‌پی مبدأ</TableHead>
                  <TableHead className='text-start'>پروتکل</TableHead>
                  <TableHead className='text-start'>پورت</TableHead>
                  <TableHead className='text-start'>سرعت ارسال</TableHead>
                  <TableHead className='text-start'>سرعت دریافت</TableHead>
                  <TableHead className='text-start'>موقعیت</TableHead>
                  <TableHead className='text-start'>اپراتور</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((flow, i) => (
                  <TableRow key={i}>
                    <TableCell className='font-mono text-xs'>
                      {flow.src_address}
                    </TableCell>
                    <TableCell>{flow.ip_protocol || '—'}</TableCell>
                    <TableCell>{flow.src_port ?? '—'}</TableCell>
                    <TableCell>{formatBytesFa(flow.tx_bytes_per_second)}/s</TableCell>
                    <TableCell>{formatBytesFa(flow.rx_bytes_per_second)}/s</TableCell>
                    <TableCell>
                      {[flow.city, flow.country].filter(Boolean).join('، ') || '—'}
                    </TableCell>
                    <TableCell>{flow.isp || '—'}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
