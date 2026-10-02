import { useQuery } from '@tanstack/react-query'
import { fetchSecurityIdentityHistory } from '@/api/security.ts'
import { formatBytesFa, formatDateTimeFa } from '@/features/reports/lib/format.ts'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { SecurityWorldMap } from '@/features/security/components/security-world-map.tsx'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  protocol: string | null
  identity: string | null
}

export function IdentityHistoryDialog({
  open,
  onOpenChange,
  protocol,
  identity,
}: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['security_identity_history', protocol, identity],
    queryFn: () => fetchSecurityIdentityHistory(protocol!, identity!),
    enabled: open && !!protocol && !!identity,
  })

  const mapPoints = (data?.sessions ?? [])
    .filter((s) => s.lat != null && s.lon != null)
    .map((s, i) => ({
      key: `${s.ip_address}-${i}`,
      lat: s.lat as number,
      lon: s.lon as number,
      label: s.ip_address,
      sublabel: [s.city, s.country].filter(Boolean).join('، ') || undefined,
    }))

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto p-4 sm:max-w-3xl sm:p-6'>
        <DialogHeader>
          <DialogTitle className='break-all'>
            تاریخچه‌ی اتصال {identity}
          </DialogTitle>
        </DialogHeader>

        {isLoading ? (
          <Skeleton className='h-64 w-full rounded-lg' />
        ) : !data || data.sessions.length === 0 ? (
          <EmptyState message='هیچ تاریخچه‌ی اتصالی برای این کاربر ثبت نشده است.' />
        ) : (
          <div className='space-y-4'>
            {mapPoints.length > 0 && <SecurityWorldMap points={mapPoints} />}

            {/* Narrow screens: a stacked card per session -- a 6-column
                table squeezed into a phone width is only reachable via
                horizontal scroll, which is what was reported as
                "not designed for mobile at all." Wide screens keep the
                table, which reads better once there's room for it. */}
            <div className='space-y-3 sm:hidden'>
              {[...data.sessions].reverse().map((session, i) => (
                <div key={i} className='space-y-2 rounded-lg border p-3 text-sm'>
                  <div className='flex items-center justify-between gap-2'>
                    <span className='font-mono text-xs'>{session.ip_address}</span>
                    {session.disconnected_at ? (
                      <span className='text-muted-foreground text-xs'>قطع‌شده</span>
                    ) : (
                      <Badge>هم‌اکنون متصل</Badge>
                    )}
                  </div>
                  <div className='text-muted-foreground grid grid-cols-2 gap-x-2 gap-y-1 text-xs'>
                    <span>موقعیت</span>
                    <span className='text-foreground text-end'>
                      {[session.city, session.country].filter(Boolean).join('، ') || '—'}
                    </span>
                    <span>اپراتور</span>
                    <span className='text-foreground text-end'>{session.isp || '—'}</span>
                    <span>زمان اتصال</span>
                    <span className='text-foreground text-end'>
                      {formatDateTimeFa(session.connected_at)}
                    </span>
                    {session.disconnected_at && (
                      <>
                        <span>زمان قطع</span>
                        <span className='text-foreground text-end'>
                          {formatDateTimeFa(session.disconnected_at)}
                        </span>
                      </>
                    )}
                    <span>حجم مصرفی</span>
                    <span className='text-foreground text-end'>
                      {formatBytesFa(session.used_bytes)}
                    </span>
                  </div>
                </div>
              ))}
            </div>

            <div className='hidden overflow-x-auto sm:block'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='text-start'>آی‌پی</TableHead>
                    <TableHead className='text-start'>موقعیت</TableHead>
                    <TableHead className='text-start'>اپراتور</TableHead>
                    <TableHead className='text-start'>زمان اتصال</TableHead>
                    <TableHead className='text-start'>زمان قطع</TableHead>
                    <TableHead className='text-start'>حجم مصرفی</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {[...data.sessions].reverse().map((session, i) => (
                    <TableRow key={i}>
                      <TableCell className='font-mono text-xs'>
                        {session.ip_address}
                      </TableCell>
                      <TableCell>
                        {[session.city, session.country]
                          .filter(Boolean)
                          .join('، ') || '—'}
                      </TableCell>
                      <TableCell>{session.isp || '—'}</TableCell>
                      <TableCell>{formatDateTimeFa(session.connected_at)}</TableCell>
                      <TableCell>
                        {session.disconnected_at ? (
                          formatDateTimeFa(session.disconnected_at)
                        ) : (
                          <Badge>هم‌اکنون متصل</Badge>
                        )}
                      </TableCell>
                      <TableCell>{formatBytesFa(session.used_bytes)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
