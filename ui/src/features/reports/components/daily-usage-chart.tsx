import { useMemo, useState } from 'react'
import { IconChartAreaLine } from '@tabler/icons-react'
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { useDailyUsageReportQuery } from '@/hooks/reports/useDailyUsageReportQuery.ts'
import { formatBytesFa, formatDateFa } from '@/features/reports/lib/format.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportEmptyState } from '@/features/reports/components/report-empty-state.tsx'

const chartConfig = {
  wireguard: {
    label: 'وایرگارد',
    color: 'var(--chart-1)',
  },
  user_manager: {
    label: 'یوزر منیجر',
    color: 'var(--chart-2)',
  },
  v2ray: {
    label: 'V2Ray',
    color: 'var(--chart-3)',
  },
  total: {
    label: 'مجموع',
    color: 'var(--chart-4)',
  },
} satisfies ChartConfig

type DailyUsageChartProps = {
  range: ReportsRange
}

// Report 1 -- daily usage over time, multi-series (WireGuard / User
// Manager / V2Ray / Total) stacked area chart with a shared legend acting
// as the "toggle which series show" affordance (Recharts' Legend already
// supports click-to-hide out of the box via ChartLegendContent).
export function DailyUsageChart({ range }: DailyUsageChartProps) {
  const { data, isLoading, isFetching } = useDailyUsageReportQuery(range)
  const [hidden, setHidden] = useState<Record<string, boolean>>({})

  const points = useMemo(
    () =>
      (data?.points ?? []).map((point) => ({
        date: point.date,
        wireguard: point.wireguard_bytes,
        user_manager: point.user_manager_bytes,
        v2ray: point.v2ray_bytes,
        total: point.total_bytes,
      })),
    [data]
  )

  const toggleSeries = (dataKey: string) => {
    setHidden((prev) => ({ ...prev, [dataKey]: !prev[dataKey] }))
  }

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 col-span-1 flex flex-col pt-0 duration-500 lg:col-span-2'>
      <CardHeader className='flex items-center gap-2 space-y-0 border-b py-5 sm:flex-row'>
        <div className='grid flex-1 gap-1'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>مصرف روزانه</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            روند مصرف ترافیک به تفکیک پروتکل در بازه انتخاب‌شده
          </p>
        </div>
      </CardHeader>

      <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
        {isLoading || isFetching ? (
          <Skeleton className='h-[280px] w-full rounded-lg' />
        ) : points.length === 0 ? (
          <ReportEmptyState
            icon={<IconChartAreaLine className='size-10 opacity-60' />}
            message='برای این بازه زمانی داده مصرفی ثبت نشده است.'
          />
        ) : (
          <ChartContainer
            config={chartConfig}
            className='aspect-auto h-[280px] w-full'
          >
            <AreaChart data={points}>
              <defs>
                {(
                  ['wireguard', 'user_manager', 'v2ray', 'total'] as const
                ).map((key) => (
                  <linearGradient
                    key={key}
                    id={`fill-${key}`}
                    x1='0'
                    y1='0'
                    x2='0'
                    y2='1'
                  >
                    <stop
                      offset='5%'
                      stopColor={`var(--color-${key})`}
                      stopOpacity={0.8}
                    />
                    <stop
                      offset='95%'
                      stopColor={`var(--color-${key})`}
                      stopOpacity={0.1}
                    />
                  </linearGradient>
                ))}
              </defs>

              <CartesianGrid vertical={false} />
              <XAxis
                dataKey='date'
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={32}
                tickFormatter={(value) => formatDateFa(value)}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                width={56}
                tickFormatter={(value) => formatBytesFa(value)}
              />

              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    labelFormatter={(value) => formatDateFa(value)}
                    formatter={(value, name) => [
                      formatBytesFa(Number(value)),
                      ' ' +
                        (chartConfig[name as keyof typeof chartConfig]
                          ?.label ?? name),
                    ]}
                    indicator='dot'
                  />
                }
              />

              {(['wireguard', 'user_manager', 'v2ray', 'total'] as const).map(
                (key) =>
                  !hidden[key] && (
                    <Area
                      key={key}
                      dataKey={key}
                      type='natural'
                      fill={`url(#fill-${key})`}
                      stroke={`var(--color-${key})`}
                      strokeWidth={2}
                    />
                  )
              )}

              <ChartLegend
                content={<ChartLegendContent />}
                onClick={(entry) => {
                  if (entry?.dataKey) toggleSeries(String(entry.dataKey))
                }}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
