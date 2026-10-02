import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'
import {
  fetchTunnelBackupProbeResults,
  fetchTunnelHealthScoreHistory,
  fetchTunnelHealthStatuses,
  fetchTunnelIncidentDiagnoses,
} from '@/api/tunnel-health.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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

const chartConfig = {
  score: {
    label: 'امتیاز سلامت',
    color: 'var(--chart-1)',
  },
} satisfies ChartConfig

// HealthScoreTrendCard is the early-warning capability's own visual
// surface: a per-tunnel selector plus a 0-100 trend line, so an admin
// investigating a problem tunnel can see whether it's been quietly
// degrading over the last samples rather than only seeing today's
// binary healthy/suspect/confirmed_down status (see StatusesTab for
// that). The engine computes and stores a score every 60s poll tick
// (service.TunnelHealthService.recordHealthScoreAndWarn) purely from
// signals it already reads -- this chart is a read-only view of that.
function HealthScoreTrendCard() {
  const { data: statuses = [] } = useQuery({
    queryKey: ['tunnel_health_statuses'],
    queryFn: () => fetchTunnelHealthStatuses(),
  })
  const [selected, setSelected] = useState<string>('')
  const interfaceName = selected || statuses[0]?.interface_name || ''

  const { data: scores = [], isLoading } = useQuery({
    queryKey: ['tunnel_health_scores', interfaceName],
    queryFn: () => fetchTunnelHealthScoreHistory(interfaceName),
    enabled: !!interfaceName,
    refetchInterval: 60_000,
  })

  const points = useMemo(
    () =>
      scores.map((s) => ({
        score: s.score,
        time: new Date(s.sampled_at).toLocaleTimeString('fa-IR', {
          hour: '2-digit',
          minute: '2-digit',
        }),
      })),
    [scores]
  )

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0'>
        <div>
          <CardTitle>روند امتیاز سلامت (هشدار زودهنگام)</CardTitle>
          <p className='text-muted-foreground mt-1 text-sm'>
            روند این تانل پیش از رسیدن به وضعیت مشکوک یا قطع‌شده -- افت
            محسوس این روند باعث ارسال هشدار زودهنگام می‌شود.
          </p>
        </div>
        {statuses.length > 0 && (
          <Select value={interfaceName} onValueChange={setSelected}>
            <SelectTrigger className='w-48'>
              <SelectValue placeholder='انتخاب تانل' />
            </SelectTrigger>
            <SelectContent>
              {statuses.map((s) => (
                <SelectItem key={s.id} value={s.interface_name}>
                  {s.interface_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </CardHeader>
      <CardContent className='px-2 pt-4 sm:px-6'>
        {!interfaceName ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تانلی کشف نشده است.
          </p>
        ) : isLoading ? (
          <Skeleton className='h-[220px] w-full rounded-lg' />
        ) : points.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز داده‌ی کافی برای رسم روند ثبت نشده است.
          </p>
        ) : (
          <ChartContainer
            config={chartConfig}
            className='aspect-auto h-[220px] w-full'
          >
            <LineChart data={points}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey='time'
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={32}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                width={36}
                domain={[0, 100]}
              />
              <ChartTooltip
                cursor={{ strokeDasharray: '4 4' }}
                content={<ChartTooltipContent />}
              />
              <Line
                dataKey='score'
                type='monotone'
                stroke='var(--color-score)'
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}

const CAUSE_LABELS_FA: Record<string, string> = {
  remote_unreachable: 'مشکل مختص همین تانل',
  router_uplink_down: 'قطعی کل اتصال روتر',
  router_resource_exhausted: 'فشار منابع روی روتر',
  unknown: 'نامشخص',
}

const CAUSE_COLORS: Record<string, 'yellow' | 'red' | 'gray'> = {
  remote_unreachable: 'yellow',
  router_uplink_down: 'red',
  router_resource_exhausted: 'red',
  unknown: 'gray',
}

// IncidentDiagnosesCard shows the self-healing engine's own root-cause
// guess for recent confirmed_down incidents (computed once per incident
// via active ping probes -- see service.TunnelHealthService.
// diagnoseIncidentOnce) so an admin doesn't have to manually investigate
// whether a down tunnel is its own isolated problem or a symptom of the
// whole router losing connectivity.
function IncidentDiagnosesCard() {
  const { data: diagnoses = [], isLoading } = useQuery({
    queryKey: ['tunnel_incident_diagnoses'],
    queryFn: () => fetchTunnelIncidentDiagnoses(),
    refetchInterval: 60_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>تشخیص علت قطعی‌ها</CardTitle>
        <p className='text-muted-foreground text-sm'>
          حدس خودکار علت هر قطعی، بر اساس تست پینگ فعال از همان روتر --
          آیا مشکل مختص همین تانل است، کل روتر قطع شده، یا خود روتر تحت
          فشار است.
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-32 w-full rounded-lg' />
        ) : diagnoses.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ قطعی‌ای تشخیص داده نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>حدس علت</TableHead>
                  <TableHead>شواهد</TableHead>
                  <TableHead>زمان</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {diagnoses.map((d) => (
                  <TableRow key={d.id}>
                    <TableCell className='font-medium'>
                      {d.interface_name}
                    </TableCell>
                    <TableCell>
                      <ColoredBadge
                        color={CAUSE_COLORS[d.cause] ?? 'gray'}
                        text={CAUSE_LABELS_FA[d.cause] ?? d.cause}
                      />
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {d.evidence}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {new Date(d.diagnosed_at).toLocaleString('fa-IR')}
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

// BackupProbeResultsCard shows the nightly active-probe results for each
// tunnel's OWN configured Level 2 backup gateway (service.
// TunnelHealthService.ProbeBackupPaths, run once daily at 03:00) -- lets
// an admin confirm a backup path actually works BEFORE a real incident
// ever needs it, instead of discovering a dead backup during a live
// double-failure.
function BackupProbeResultsCard() {
  const { data: probes = [], isLoading } = useQuery({
    queryKey: ['tunnel_backup_probe_results'],
    queryFn: () => fetchTunnelBackupProbeResults(),
    refetchInterval: 60_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>تست شبانه مسیر بکاپ</CardTitle>
        <p className='text-muted-foreground text-sm'>
          هر شب ساعت ۳ بامداد، آی‌پی واسطه‌ی بکاپ هر تانل (سطح ۲) به‌صورت
          خودکار پینگ می‌شود تا از سالم بودن مسیر جایگزین پیش از وقوع
          قطعی واقعی اطمینان حاصل شود.
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-32 w-full rounded-lg' />
        ) : probes.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تانلی با آی‌پی واسطه‌ی بکاپ تنظیم‌شده تست نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>آی‌پی بکاپ</TableHead>
                  <TableHead>وضعیت</TableHead>
                  <TableHead>شواهد</TableHead>
                  <TableHead>زمان تست</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {probes.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className='font-medium'>
                      {p.interface_name}
                    </TableCell>
                    <TableCell className='font-mono text-sm'>
                      {p.backup_gateway_ip}
                    </TableCell>
                    <TableCell>
                      <ColoredBadge
                        color={p.reachable ? 'green' : 'red'}
                        text={p.reachable ? 'در دسترس' : 'در دسترس نیست'}
                      />
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {p.evidence}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {new Date(p.probed_at).toLocaleString('fa-IR')}
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

// AnalysisTab groups the three predictive/analytical capabilities added
// alongside the original detect-and-react engine: early-warning health
// trend, active root-cause diagnosis, and nightly backup-path
// pre-checks. The fourth capability (the weekly incident-pattern digest)
// is Telegram-only by design -- an analytical summary sent once a week,
// not a live page an admin would refresh (see service.
// TunnelHealthService.BuildIncidentPatternReport's own doc comment).
export function AnalysisTab() {
  return (
    <div className='space-y-4'>
      <HealthScoreTrendCard />
      <IncidentDiagnosesCard />
      <BackupProbeResultsCard />
    </div>
  )
}
