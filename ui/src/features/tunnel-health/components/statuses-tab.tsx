import { useQuery } from '@tanstack/react-query'
import { fetchTunnelHealthStatuses } from '@/api/tunnel-health.ts'
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

function formatHandshakeAge(seconds: number | null | undefined): string {
  if (seconds == null) return '—'
  if (seconds < 60) return `${seconds} ثانیه پیش`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} دقیقه پیش`
  return `${Math.floor(seconds / 3600)} ساعت پیش`
}

// StatusesTab shows every DISCOVERED INFRASTRUCTURE TUNNEL's (GRE/IPIP/
// EoIP/WireGuard-as-link between servers -- never a customer-facing
// WireGuard peer interface, see service.TunnelGraphService.
// DiscoveredTunnelInterfaces' own doc comment for how these are told
// apart) CURRENT computed health -- refetched every 30s so an admin
// watching this page sees a state change without a manual reload.
export function StatusesTab() {
  const { data: rows = [], isLoading } = useQuery({
    queryKey: ['tunnel_health_statuses'],
    queryFn: () => fetchTunnelHealthStatuses(),
    refetchInterval: 30_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>وضعیت فعلی تانل‌ها</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تانل زیرساختی کشف نشده است. کشف هر ۵ دقیقه یک‌بار
            خودکار اجرا می‌شود.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>نوع</TableHead>
                  <TableHead>وضعیت</TableHead>
                  <TableHead>در حال اجرا</TableHead>
                  <TableHead>آخرین handshake</TableHead>
                  <TableHead>نامتقارنی Tx/Rx</TableHead>
                  <TableHead>آخرین بررسی</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className='font-medium'>
                      {row.interface_name}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {row.interface_type ?? '—'}
                    </TableCell>
                    <TableCell>
                      <ColoredBadge
                        color={SEVERITY_COLORS[row.severity] ?? 'gray'}
                        text={SEVERITY_LABELS_FA[row.severity] ?? row.severity}
                      />
                    </TableCell>
                    <TableCell>
                      {row.interface_disabled
                        ? 'غیرفعال'
                        : row.interface_running
                          ? 'بله'
                          : 'خیر'}
                    </TableCell>
                    <TableCell>
                      {formatHandshakeAge(
                        row.worst_peer_last_handshake_age_seconds
                      )}
                    </TableCell>
                    <TableCell>
                      {row.tx_rx_asymmetry_detected ? 'دارد' : 'ندارد'}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {new Date(row.last_polled_at).toLocaleString('fa-IR')}
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
