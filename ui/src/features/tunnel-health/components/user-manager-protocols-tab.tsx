import { useQuery } from '@tanstack/react-query'
import {
  fetchUserManagerProtocolEvents,
  fetchUserManagerProtocolStatuses,
} from '@/api/tunnel-health.ts'
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
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'

const PROTOCOL_LABELS_FA: Record<string, string> = {
  l2tp: 'L2TP',
  pptp: 'PPTP',
  sstp: 'SSTP',
  openvpn: 'OpenVPN',
}

// UserManagerProtocolsTab is spec section ب-7's own dedicated row for
// L2TP/PPTP/SSTP/OpenVPN -- deliberately ALERT-ONLY (no toggle/action
// buttons here at all): restarting one of these services could disrupt
// active customer sessions, so this page only surfaces the hybrid
// router-enabled + TCP-reachable check's own results.
export function UserManagerProtocolsTab() {
  const { data: statuses = [], isLoading: statusesLoading } = useQuery({
    queryKey: ['user_manager_protocol_statuses'],
    queryFn: () => fetchUserManagerProtocolStatuses(),
    refetchInterval: 30_000,
  })
  const { data: events = [], isLoading: eventsLoading } = useQuery({
    queryKey: ['user_manager_protocol_events'],
    queryFn: () => fetchUserManagerProtocolEvents(100),
    refetchInterval: 30_000,
  })

  return (
    <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>وضعیت فعلی پروتکل‌ها</CardTitle>
        </CardHeader>
        <CardContent>
          {statusesLoading ? (
            <Skeleton className='h-32 w-full rounded-lg' />
          ) : statuses.length === 0 ? (
            <p className='text-muted-foreground py-8 text-center text-sm'>
              هیچ پروتکلی در تنظیمات User Manager فعال نشده است.
            </p>
          ) : (
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>پروتکل</TableHead>
                    <TableHead>وضعیت</TableHead>
                    <TableHead>فعال در روتر</TableHead>
                    <TableHead>پورت پاسخگو</TableHead>
                    <TableHead>آخرین بررسی</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {statuses.map((row) => (
                    <TableRow key={row.protocol}>
                      <TableCell className='font-medium'>
                        {PROTOCOL_LABELS_FA[row.protocol] ?? row.protocol}
                      </TableCell>
                      <TableCell>
                        <ColoredBadge
                          color={row.healthy ? 'green' : 'red'}
                          text={row.healthy ? 'سالم' : 'قطع'}
                        />
                      </TableCell>
                      <TableCell>{row.router_enabled ? 'بله' : 'خیر'}</TableCell>
                      <TableCell>{row.port_reachable ? 'بله' : 'خیر'}</TableCell>
                      <TableCell className='text-muted-foreground text-sm'>
                        {new Date(row.last_checked_at).toLocaleString('fa-IR')}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>تاریخچه رویدادها</CardTitle>
        </CardHeader>
        <CardContent>
          {eventsLoading ? (
            <Skeleton className='h-32 w-full rounded-lg' />
          ) : events.length === 0 ? (
            <p className='text-muted-foreground py-8 text-center text-sm'>
              هنوز هیچ تغییر وضعیتی ثبت نشده است.
            </p>
          ) : (
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>پروتکل</TableHead>
                    <TableHead>از</TableHead>
                    <TableHead>به</TableHead>
                    <TableHead>شواهد</TableHead>
                    <TableHead>هشدار تلگرام</TableHead>
                    <TableHead>زمان</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {events.map((row) => (
                    <TableRow key={row.id}>
                      <TableCell className='font-medium'>
                        {PROTOCOL_LABELS_FA[row.protocol] ?? row.protocol}
                      </TableCell>
                      <TableCell>
                        {row.from_status === 'healthy' ? 'سالم' : 'قطع'}
                      </TableCell>
                      <TableCell>
                        {row.to_status === 'healthy' ? 'سالم' : 'قطع'}
                      </TableCell>
                      <TableCell className='max-w-md text-sm'>
                        {row.evidence}
                      </TableCell>
                      <TableCell>
                        {row.notified_at ? 'ارسال شد' : '—'}
                      </TableCell>
                      <TableCell className='text-muted-foreground text-sm'>
                        {new Date(row.detected_at).toLocaleString('fa-IR')}
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
