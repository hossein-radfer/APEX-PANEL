import { IconAlertTriangle } from '@tabler/icons-react'
import { useAnomalyAlertsReportQuery } from '@/hooks/reports/useAnomalyAlertsReportQuery.ts'
import { formatBytesFa, protocolLabelFa } from '@/features/reports/lib/format.ts'
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

// Report 13 -- always "right now": users whose usage today is an unusual
// multiple of their own recent daily average.
export function AnomalyAlertsTable() {
  const { data, isLoading } = useAnomalyAlertsReportQuery()

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='space-y-0'>
        <CardTitle>
          <h2 className='text-lg font-semibold'>هشدارهای مصرف غیرعادی</h2>
        </CardTitle>
        <p className='text-muted-foreground text-sm'>
          کاربرانی با مصرف امروز به‌طور غیرعادی بالاتر از میانگین همیشگی
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-[220px] w-full rounded-lg' />
        ) : !data || data.alerts.length === 0 ? (
          <ReportEmptyState
            icon={<IconAlertTriangle className='size-10 opacity-60' />}
            message='هیچ رفتار غیرعادی‌ای شناسایی نشده است.'
          />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-start'>نام</TableHead>
                  <TableHead className='text-start'>پروتکل</TableHead>
                  <TableHead className='text-start'>مصرف امروز</TableHead>
                  <TableHead className='text-start'>میانگین مصرف</TableHead>
                  <TableHead className='text-start'>ضریب افزایش</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.alerts.map((alert, index) => (
                  <TableRow key={`${alert.name}-${index}`}>
                    <TableCell className='font-medium'>
                      {alert.name}
                    </TableCell>
                    <TableCell>{protocolLabelFa(alert.protocol)}</TableCell>
                    <TableCell>{formatBytesFa(alert.today_bytes)}</TableCell>
                    <TableCell>
                      {formatBytesFa(alert.average_bytes)}
                    </TableCell>
                    <TableCell>
                      <Badge variant='destructive'>
                        ×{alert.multiplier.toLocaleString('fa-IR', {
                          maximumFractionDigits: 1,
                        })}
                      </Badge>
                    </TableCell>
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
