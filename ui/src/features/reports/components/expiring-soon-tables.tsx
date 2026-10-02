import { IconClockExclamation } from '@tabler/icons-react'
import { useExpiringSoonReportQuery } from '@/hooks/reports/useExpiringSoonReportQuery.ts'
import {
  formatBytesFa,
  formatDateFa,
  formatPercentFa,
  protocolLabelFa,
} from '@/features/reports/lib/format.ts'
import { ReportsExpiringEntity } from '@/schema/reports.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ReportEmptyState } from '@/features/reports/components/report-empty-state.tsx'

function DaysRemainingBadge({ days }: { days: number | null | undefined }) {
  if (days === null || days === undefined) return <span>—</span>
  const variant = days <= 1 ? 'destructive' : days <= 3 ? 'secondary' : 'outline'
  return (
    <Badge variant={variant}>
      {days.toLocaleString('fa-IR')} روز
    </Badge>
  )
}

function UsagePercentBadge({ percent }: { percent: number | null | undefined }) {
  if (percent === null || percent === undefined) return <span>—</span>
  const variant =
    percent >= 95 ? 'destructive' : percent >= 85 ? 'secondary' : 'outline'
  return <Badge variant={variant}>{formatPercentFa(percent)}</Badge>
}

function ExpiringTable({
  rows,
  mode,
}: {
  rows: ReportsExpiringEntity[]
  mode: 'date' | 'volume'
}) {
  if (rows.length === 0) {
    return (
      <ReportEmptyState
        icon={<IconClockExclamation className='size-10 opacity-60' />}
        message='در حال حاضر هیچ موردی در آستانه انقضا نیست.'
      />
    )
  }

  return (
    <div className='overflow-x-auto'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='text-start'>نام</TableHead>
            <TableHead className='text-start'>پروتکل</TableHead>
            {mode === 'date' ? (
              <>
                <TableHead className='text-start'>تاریخ انقضا</TableHead>
                <TableHead className='text-start'>روز باقی‌مانده</TableHead>
              </>
            ) : (
              <>
                <TableHead className='text-start'>درصد مصرف حجم</TableHead>
                <TableHead className='text-start'>حجم باقی‌مانده</TableHead>
              </>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={`${row.id}-${row.protocol}-${mode}`}>
              <TableCell className='font-medium'>{row.name}</TableCell>
              <TableCell>{protocolLabelFa(row.protocol)}</TableCell>
              {mode === 'date' ? (
                <>
                  <TableCell>{formatDateFa(row.expire_at)}</TableCell>
                  <TableCell>
                    <DaysRemainingBadge days={row.days_remaining} />
                  </TableCell>
                </>
              ) : (
                <>
                  <TableCell>
                    <UsagePercentBadge percent={row.usage_percent} />
                  </TableCell>
                  <TableCell>{formatBytesFa(row.remaining_bytes)}</TableCell>
                </>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

// Report 6 -- always "right now" (no range param): two lists of accounts
// approaching expiry, one sorted by soonest expiry date, one by highest
// volume-usage percentage.
export function ExpiringSoonTables() {
  const { data, isLoading } = useExpiringSoonReportQuery()

  return (
    <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
      <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
        <CardHeader className='space-y-0'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>نزدیک به انقضا (زمان)</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            حساب‌هایی که به‌زودی از نظر تاریخ منقضی می‌شوند
          </p>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className='h-[220px] w-full rounded-lg' />
          ) : (
            <ExpiringTable
              rows={data?.expiring_by_soon_date ?? []}
              mode='date'
            />
          )}
        </CardContent>
      </Card>

      <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
        <CardHeader className='space-y-0'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>نزدیک به انقضا (حجم)</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            حساب‌هایی که به‌زودی حجم آن‌ها به پایان می‌رسد
          </p>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className='h-[220px] w-full rounded-lg' />
          ) : (
            <ExpiringTable rows={data?.expiring_by_volume ?? []} mode='volume' />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
