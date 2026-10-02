import { useQuery } from '@tanstack/react-query'
import { fetchTunnelMap } from '@/api/tunnel-health.ts'
import type {
  NatRuleSummary,
  TunnelMapTunnel,
} from '@/schema/tunnel-health.ts'
import { useIsMobile } from '@/hooks/use-mobile.tsx'
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

const PROTOCOL_LABELS_FA: Record<string, string> = {
  gre: 'GRE',
  ipip: 'IPIP',
  eoip: 'EoIP',
  wireguard: 'WireGuard',
}

// NatRuleFlow draws one NAT rule as the path a packet actually takes --
// port hit on the tunnel, dst-nat target, mangle mark, routing table --
// instead of the flat "پورت X ← IP:Y / مسیر: نشان مانگل «Z» ← جدول
// روتینگ «W»" text line the admin explicitly said was unreadable ("تو
// فهمیدی اصلا چی شد؟" pasted against exactly this text). Each present
// field becomes one box; only the arrows between fields that both exist
// are drawn, so a rule with just a port+target (no mangle) still reads
// cleanly as a two-box hop instead of leaving broken connector stubs.
function NatRuleFlow({ rule }: { rule: NatRuleSummary }) {
  const stages: { label: string; value: string }[] = []
  if (rule.port) stages.push({ label: 'پورت ورودی', value: rule.port })
  if (rule.target) stages.push({ label: 'مقصد', value: rule.target })
  if (rule.mangle_routing_mark)
    stages.push({ label: 'نشان مانگل', value: rule.mangle_routing_mark })
  if (rule.routing_table)
    stages.push({ label: 'جدول روتینگ', value: rule.routing_table })

  if (stages.length === 0) {
    return (
      <span className='text-muted-foreground text-sm'>
        {rule.comment || '(بدون جزئیات بیشتر)'}
      </span>
    )
  }

  const boxWidth = 132
  const boxHeight = 44
  const gap = 36
  const width = stages.length * boxWidth + (stages.length - 1) * gap
  const height = rule.comment ? 76 : 56

  // A confirmed, reported bug ("زوم زیادی نیاز داره" -- needs a lot of
  // zoom): the previous version capped maxWidth at Math.min(width, 560),
  // meaning a 3-4 stage rule (up to 636px of intrinsic viewBox width) was
  // still rendered up to 560px wide regardless of the actual viewport --
  // on a ~360-400px mobile screen that forced horizontal overflow the
  // admin had to pinch-zoom out to see, then zoom back in to read the
  // 10-12px SVG text. Removing the fixed pixel cap and relying purely on
  // `max-w-full` (scales the whole viewBox down to the CONTAINER's real
  // width, SVG text included) fixes this at every screen size with no
  // separate mobile-specific branch needed for this element specifically.
  return (
    <figure className='m-0 w-full'>
      {rule.comment && (
        <figcaption className='mb-1 text-sm font-medium'>
          {rule.comment}
        </figcaption>
      )}
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className='h-auto max-w-full'
        role='img'
        aria-label={`مسیر بسته: ${stages.map((s) => `${s.label} ${s.value}`).join(' سپس ')}`}
      >
        <defs>
          <marker
            id={`nat-arrow-${rule.port}-${rule.target}`}
            viewBox='0 0 8 8'
            refX='7'
            refY='4'
            markerWidth='6'
            markerHeight='6'
            orient='auto-start-reverse'
          >
            <polygon points='0,0 8,4 0,8' fill='currentColor' />
          </marker>
        </defs>
        {stages.map((stage, i) => {
          const x = i * (boxWidth + gap)
          const y = (height - boxHeight) / 2
          return (
            <g key={stage.label}>
              <rect
                x={x}
                y={y}
                width={boxWidth}
                height={boxHeight}
                rx={8}
                fill='none'
                stroke='currentColor'
                strokeWidth={1.5}
                opacity={0.5}
              />
              <text
                x={x + boxWidth / 2}
                y={y + 17}
                textAnchor='middle'
                fontSize={10}
                fill='currentColor'
                opacity={0.65}
              >
                {stage.label}
              </text>
              <text
                x={x + boxWidth / 2}
                y={y + 32}
                textAnchor='middle'
                fontSize={12}
                fontFamily='ui-monospace, monospace'
                fill='currentColor'
              >
                {stage.value}
              </text>
              {i < stages.length - 1 && (
                <line
                  x1={x + boxWidth}
                  y1={y + boxHeight / 2}
                  x2={x + boxWidth + gap}
                  y2={y + boxHeight / 2}
                  stroke='currentColor'
                  strokeWidth={1.5}
                  markerEnd={`url(#nat-arrow-${rule.port}-${rule.target})`}
                />
              )}
            </g>
          )
        })}
      </svg>
    </figure>
  )
}

// TunnelNatRulesCell renders one tunnel's NAT-rule column content --
// shared between the desktop table and the mobile card layout below so
// the "no rules resolved" explanation and rule list never drift apart
// between the two render paths.
//
// A confirmed, reported complaint ("بعضی تانل ها هیچ داده ای رو نشون
// نمیده" -- some tunnels show no data): this is NOT a data-fetching bug
// -- BuildTunnelMap (api/service/tunnel_map.go) deliberately never
// fabricates a NAT-rule chain it can't actually verify on the router
// (NAT rule -> mangle rule -> routing table -> route -> gateway, every
// hop confirmed real). A tunnel interface that genuinely has no NAT rule
// whose full chain resolves back to it will legitimately have zero
// entries here. The previous version showed the exact same generic "no
// direct NAT rule found" text regardless of cause, which read as "this
// is broken" rather than "this interface simply isn't NAT-mapped" --
// the added second line makes that distinction explicit instead of
// leaving the admin to guess.
function TunnelNatRulesCell({ tunnel }: { tunnel: TunnelMapTunnel }) {
  if (tunnel.nat_rules.length === 0) {
    return (
      <div className='text-muted-foreground text-sm'>
        <p>هیچ قانون NAT مستقیمی یافت نشد</p>
        <p className='mt-0.5 text-xs opacity-75'>
          یعنی هیچ زنجیره‌ی NAT ← مانگل ← جدول روتینگ ← مسیر کاملی به این
          اینترفیس پیدا نشد؛ به این معنا نیست که خود تانل مشکل دارد.
        </p>
      </div>
    )
  }
  return (
    <div className='space-y-4'>
      {tunnel.nat_rules.map((rule, i) => (
        <NatRuleFlow key={i} rule={rule} />
      ))}
    </div>
  )
}

// GraphTab is the admin's own explicit correction, answered directly:
// "خودش باید نقشه‌سازی کنه بر اساس هر پروتکل و لوکیشین و تانل... نه
// اینکه بیاد صرفا یک لیست ساده بسازه" (it should build the map itself,
// grouped by protocol/location/tunnel -- not just produce a flat list).
// This reads the server-COMPUTED hierarchy (service.TunnelGraphService.
// BuildTunnelMap) directly -- no client-side node/edge reassembly here
// at all, matching that explicit requirement.
export function GraphTab() {
  const isMobile = useIsMobile()
  const { data: map, isLoading } = useQuery({
    queryKey: ['tunnel_ai_map'],
    queryFn: () => fetchTunnelMap(),
    refetchInterval: 60_000,
  })

  if (isLoading) {
    return <Skeleton className='h-40 w-full rounded-lg' />
  }

  if (!map || map.protocols.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>گراف وابستگی</CardTitle>
        </CardHeader>
        <CardContent>
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تانل زیرساختی کشف نشده است. این بخش هر ۵ دقیقه یک‌بار
            به‌طور خودکار به‌روزرسانی می‌شود.
          </p>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        آخرین بروزرسانی نقشه:{' '}
        {new Date(map.snapshot_taken_at).toLocaleString('fa-IR')}
      </p>
      {map.protocols.map((protocolGroup) => (
        <Card key={protocolGroup.protocol}>
          <CardHeader>
            <CardTitle>
              {PROTOCOL_LABELS_FA[protocolGroup.protocol] ??
                protocolGroup.protocol}
            </CardTitle>
          </CardHeader>
          <CardContent className='space-y-4'>
            {protocolGroup.locations.map((locationGroup) => (
              <div key={locationGroup.location}>
                <p className='mb-2 text-sm font-medium'>
                  {locationGroup.location}
                  <Badge variant='secondary' className='mr-2'>
                    {locationGroup.tunnels.length}
                  </Badge>
                </p>
                {isMobile ? (
                  // A confirmed, reported complaint ("با موبایل سازگار
                  // نیست" -- not mobile compatible): the table layout
                  // below this branch has no responsive breakpoint at
                  // all -- every cell forces whitespace-nowrap (see
                  // components/ui/table.tsx, a SHARED primitive used
                  // across dozens of other features, so it's
                  // deliberately not changed globally here) and the
                  // interface-name + NAT-rule-diagram columns side by
                  // side simply don't fit a ~360-400px phone screen.
                  // Below the project's existing 768px mobile breakpoint
                  // (useIsMobile, already used by the app sidebar but
                  // never by a data table until now), each tunnel
                  // becomes its own stacked card instead -- full width,
                  // no forced single-line cells, same information as the
                  // desktop table.
                  <div className='space-y-3'>
                    {locationGroup.tunnels.map((tunnel) => (
                      <div
                        key={tunnel.interface_name}
                        className='rounded-md border p-3'
                      >
                        <p className='font-mono text-sm font-medium'>
                          {tunnel.interface_name}
                        </p>
                        <div className='mt-2'>
                          <TunnelNatRulesCell tunnel={tunnel} />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='overflow-x-auto rounded-md border'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>اینترفیس</TableHead>
                          <TableHead>قوانین NAT مرتبط</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {locationGroup.tunnels.map((tunnel) => (
                          <TableRow key={tunnel.interface_name}>
                            <TableCell className='font-mono text-sm whitespace-nowrap align-top'>
                              {tunnel.interface_name}
                            </TableCell>
                            <TableCell className='text-sm whitespace-normal'>
                              <TunnelNatRulesCell tunnel={tunnel} />
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
