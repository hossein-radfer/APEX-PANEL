import { IconUsers, IconWifi } from '@tabler/icons-react'
import { useOnlineUsersReportQuery } from '@/hooks/reports/useOnlineUsersReportQuery.ts'
import { protocolLabelFa } from '@/features/reports/lib/format.ts'
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
import { ReportStatCard } from '@/features/reports/components/report-stat-card.tsx'

// Report 8 -- live "who's online right now" data: a pulsing-dot stat card
// for the headline count plus the live user list, refetched every 15s by
// useOnlineUsersReportQuery (no range param -- always "right now").
export function OnlineUsersCard() {
  const { data, isLoading } = useOnlineUsersReportQuery()

  return (
    <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
      <ReportStatCard
        title='کاربران آنلاین'
        icon={<IconUsers />}
        value={data?.total_online}
        isLoading={isLoading}
        live
        subtitle='به‌روزرسانی هر ۱۵ ثانیه'
      />

      <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500 lg:col-span-2'>
        <CardHeader className='space-y-0'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>فهرست کاربران آنلاین</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            کاربرانی که هم‌اکنون به سرورها متصل هستند
          </p>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <Skeleton className='h-[220px] w-full rounded-lg' />
          ) : !data || data.users.length === 0 ? (
            <ReportEmptyState
              icon={<IconWifi className='size-10 opacity-60' />}
              message='در حال حاضر هیچ کاربری آنلاین نیست.'
            />
          ) : (
            <div className='max-h-[320px] overflow-y-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='text-start'>نام</TableHead>
                    <TableHead className='text-start'>پروتکل</TableHead>
                    <TableHead className='text-start'>موقعیت</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.users.map((user, index) => (
                    <TableRow key={`${user.name}-${index}`}>
                      <TableCell className='font-medium'>
                        <bdi>{user.name}</bdi>
                      </TableCell>
                      <TableCell>{protocolLabelFa(user.protocol)}</TableCell>
                      <TableCell>
                        {/* Confirmed, reported bug: a plain-LTR value here
                            (e.g. a User Manager group literally named
                            "default") rendered with its first character
                            clipped ("efault") -- the browser's bidi
                            algorithm can visually mis-render a
                            direction-less run of Latin text sitting
                            un-isolated inside this RTL table cell. <bdi>
                            isolates the value's own directionality from
                            the surrounding RTL context regardless of
                            whether it turns out to be Latin or Persian. */}
                        <bdi>{user.location || '—'}</bdi>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
