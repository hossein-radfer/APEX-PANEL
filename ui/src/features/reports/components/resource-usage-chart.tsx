import { useMemo } from 'react'
import { IconCpu } from '@tabler/icons-react'
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'
import { useResourceUsageReportQuery } from '@/hooks/reports/useResourceUsageReportQuery.ts'
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
import { ReportStatCard } from '@/features/reports/components/report-stat-card.tsx'

const chartConfig = {
  cpu_load_percent: {
    label: 'مصرف پردازنده',
    color: 'var(--chart-1)',
  },
  memory_used_percent: {
    label: 'مصرف حافظه',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

type ResourceUsageChartProps = {
  range: ReportsRange
}

// Report 15 -- Mikrotik router CPU/memory history line chart, plus two
// stat cards for the peak readings in the range.
export function ResourceUsageChart({ range }: ResourceUsageChartProps) {
  const { data, isLoading, isFetching } = useResourceUsageReportQuery(range)

  const points = useMemo(
    () =>
      (data?.points ?? []).map((point) => ({
        ...point,
        time: new Date(point.timestamp * 1000).toLocaleTimeString('fa-IR', {
          hour: '2-digit',
          minute: '2-digit',
        }),
      })),
    [data]
  )

  return (
    <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:col-span-3 lg:grid-cols-3'>
        <ReportStatCard
          title='بیشینه مصرف پردازنده'
          icon={<IconCpu />}
          value={data?.peak_cpu_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
        <ReportStatCard
          title='میانگین مصرف پردازنده'
          icon={<IconCpu />}
          value={data?.average_cpu_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
        <ReportStatCard
          title='کمینه مصرف پردازنده'
          icon={<IconCpu />}
          value={data?.lowest_cpu_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
        <ReportStatCard
          title='بیشینه مصرف حافظه'
          icon={<IconCpu />}
          value={data?.peak_memory_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
        <ReportStatCard
          title='میانگین مصرف حافظه'
          icon={<IconCpu />}
          value={data?.average_memory_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
        <ReportStatCard
          title='کمینه مصرف حافظه'
          icon={<IconCpu />}
          value={data?.lowest_memory_percent}
          isLoading={isLoading}
          suffix='٪'
          decimals={1}
        />
      </div>

      <Card className='animate-in fade-in slide-in-from-bottom-2 flex flex-col pt-0 duration-500 lg:col-span-3'>
        <CardHeader className='space-y-0 border-b py-5'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>روند مصرف منابع روتر</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            تاریخچه مصرف پردازنده و حافظه میکروتیک
          </p>
        </CardHeader>
        <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
          {isLoading || isFetching ? (
            <Skeleton className='h-[240px] w-full rounded-lg' />
          ) : points.length === 0 ? (
            <ReportEmptyState message='داده منابع روتر برای این بازه ثبت نشده است.' />
          ) : (
            <ChartContainer
              config={chartConfig}
              className='aspect-auto h-[240px] w-full'
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
                  width={40}
                  domain={[0, 100]}
                  tickFormatter={(value) => `٪${value}`}
                />
                <ChartTooltip
                  cursor={{ strokeDasharray: '4 4' }}
                  content={
                    <ChartTooltipContent
                      formatter={(value, name) => [
                        `٪${Number(value).toLocaleString('fa-IR', { maximumFractionDigits: 1 })}`,
                        ' ' +
                          (chartConfig[name as keyof typeof chartConfig]
                            ?.label ?? name),
                      ]}
                    />
                  }
                />
                <Line
                  dataKey='cpu_load_percent'
                  type='monotone'
                  stroke='var(--color-cpu_load_percent)'
                  strokeWidth={2}
                  dot={false}
                />
                <Line
                  dataKey='memory_used_percent'
                  type='monotone'
                  stroke='var(--color-memory_used_percent)'
                  strokeWidth={2}
                  dot={false}
                />
                <ChartLegend content={<ChartLegendContent />} />
              </LineChart>
            </ChartContainer>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
