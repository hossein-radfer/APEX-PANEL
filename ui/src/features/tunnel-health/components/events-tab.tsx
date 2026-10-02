import { useQuery } from '@tanstack/react-query'
import { fetchTunnelHealthEvents } from '@/api/tunnel-health.ts'
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

const SEVERITY_LABELS_FA: Record<string, string> = {
  healthy: 'سالم',
  suspect: 'مشکوک',
  confirmed_down: 'قطع تایید شده',
}

const SEVERITY_COLORS: Record<string, 'green' | 'yellow' | 'red'> = {
  healthy: 'green',
  suspect: 'yellow',
  confirmed_down: 'red',
}

// EventsTab is the append-only history of every severity TRANSITION (see
// model.TunnelHealthEvent's own doc comment) -- one row per incident, not
// per poll tick. notified_at shows whether this specific transition
// actually triggered a Telegram alert (only a transition INTO
// suspect/confirmed_down does, per the admin's own explicit choice).
export function EventsTab() {
  const { data: rows = [], isLoading } = useQuery({
    queryKey: ['tunnel_health_events'],
    queryFn: () => fetchTunnelHealthEvents(200),
    refetchInterval: 30_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>تاریخچه رویدادها</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تغییر وضعیتی ثبت نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>از</TableHead>
                  <TableHead>به</TableHead>
                  <TableHead>شواهد</TableHead>
                  <TableHead>هشدار تلگرام</TableHead>
                  <TableHead>زمان</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className='font-medium'>
                      {row.interface_name}
                    </TableCell>
                    <TableCell>
                      <ColoredBadge
                        color={SEVERITY_COLORS[row.from_status] ?? 'gray'}
                        text={
                          SEVERITY_LABELS_FA[row.from_status] ??
                          row.from_status
                        }
                      />
                    </TableCell>
                    <TableCell>
                      <ColoredBadge
                        color={SEVERITY_COLORS[row.to_status] ?? 'gray'}
                        text={
                          SEVERITY_LABELS_FA[row.to_status] ?? row.to_status
                        }
                      />
                    </TableCell>
                    <TableCell className='max-w-md text-sm'>
                      {row.evidence}
                    </TableCell>
                    <TableCell>{row.notified_at ? 'ارسال شد' : '—'}</TableCell>
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
  )
}
