import { IconServerBolt } from '@tabler/icons-react'
import { usePanelHealthReportQuery } from '@/hooks/reports/usePanelHealthReportQuery.ts'
import { formatDateTimeFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
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

// Report 9 -- always "right now": each XuiPanel's own sync/health status.
export function PanelHealthTable() {
  const { data, isLoading } = usePanelHealthReportQuery()

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='space-y-0'>
        <CardTitle>
          <h2 className='text-lg font-semibold'>سلامت پنل‌ها</h2>
        </CardTitle>
        <p className='text-muted-foreground text-sm'>
          وضعیت همگام‌سازی و خطاهای اخیر هر پنل X-UI
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-[220px] w-full rounded-lg' />
        ) : !data || data.rows.length === 0 ? (
          <ReportEmptyState
            icon={<IconServerBolt className='size-10 opacity-60' />}
            message='هیچ پنلی برای نمایش وجود ندارد.'
          />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-start'>نام پنل</TableHead>
                  <TableHead className='text-start'>آخرین همگام‌سازی</TableHead>
                  <TableHead className='text-start'>خطاهای اخیر</TableHead>
                  <TableHead className='text-start'>تعداد موقعیت‌ها</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.rows.map((row) => (
                  <TableRow key={row.panel_id}>
                    <TableCell className='font-medium'>
                      {row.panel_name}
                    </TableCell>
                    <TableCell>
                      {formatDateTimeFa(row.last_synced_at)}
                    </TableCell>
                    <TableCell>
                      {row.recent_error_count > 0 &&
                      row.recent_errors &&
                      row.recent_errors.length > 0 ? (
                        <Popover>
                          <PopoverTrigger asChild>
                            <button type='button'>
                              <Badge
                                variant='destructive'
                                className='cursor-pointer'
                              >
                                {row.recent_error_count.toLocaleString(
                                  'fa-IR'
                                )}
                              </Badge>
                            </button>
                          </PopoverTrigger>
                          <PopoverContent
                            className='w-96 space-y-2'
                            align='start'
                          >
                            <p className='text-sm font-medium'>
                              خطاهای اخیر {row.panel_name}
                            </p>
                            <ul className='max-h-64 space-y-1 overflow-y-auto text-xs'>
                              {row.recent_errors.map((err, i) => (
                                <li
                                  key={i}
                                  className='text-muted-foreground border-b pb-1 last:border-0'
                                >
                                  {err}
                                </li>
                              ))}
                            </ul>
                          </PopoverContent>
                        </Popover>
                      ) : (
                        <Badge variant='outline'>
                          {row.recent_error_count.toLocaleString('fa-IR')}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      {row.location_count.toLocaleString('fa-IR')}
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
