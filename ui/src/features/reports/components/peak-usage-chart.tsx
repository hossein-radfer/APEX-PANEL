import { useMemo, useState } from 'react'
import { IconClock, IconSearch, IconTrendingUp } from '@tabler/icons-react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  XAxis,
  YAxis,
} from 'recharts'
import { usePeakUsageReportQuery } from '@/hooks/reports/usePeakUsageReportQuery.ts'
import { formatBytesFa, formatDateFa } from '@/features/reports/lib/format.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportEmptyState } from '@/features/reports/components/report-empty-state.tsx'

const chartConfig = {
  bytes: {
    label: 'حداکثر مصرف',
    color: 'var(--chart-1)',
  },
} satisfies ChartConfig

const hourlyChartConfig = {
  bytes: {
    label: 'مصرف ساعتی',
    color: 'var(--chart-2)',
  },
} satisfies ChartConfig

// Renders an hour (0-23) as a two-digit Persian-digit "HH:00" label.
function formatHourFa(hour: number): string {
  const hourStr = hour.toLocaleString('fa-IR', { minimumIntegerDigits: 2 })
  return `${hourStr}:۰۰`
}

type PeakUsageChartProps = {
  range: ReportsRange
}

// Report 2 -- system-wide (or, when searched, one specific entity's) peak
// usage reading per day as a line chart, plus a highlighted single peak
// figure summarizing the whole series.
export function PeakUsageChart({ range }: PeakUsageChartProps) {
  const [entity, setEntity] = useState('')
  const [debouncedEntity, setDebouncedEntity] = useState('')

  const { data, isLoading, isFetching } = usePeakUsageReportQuery(
    range,
    debouncedEntity || undefined
  )

  const points = useMemo(() => data?.points ?? [], [data])

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 flex flex-col pt-0 duration-500'>
      <CardHeader className='flex flex-col gap-3 space-y-0 border-b py-5 sm:flex-row sm:items-center'>
        <div className='grid flex-1 gap-1'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>اوج مصرف</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            بیشترین میزان مصرف ثبت‌شده در هر روز
          </p>
        </div>
        <div className='relative w-full sm:w-56'>
          <IconSearch className='text-muted-foreground pointer-events-none absolute end-2.5 top-1/2 size-4 -translate-y-1/2' />
          <Input
            placeholder='جستجوی کاربر یا نماینده...'
            value={entity}
            onChange={(event) => {
              setEntity(event.target.value)
              setDebouncedEntity(event.target.value)
            }}
            className='pe-9'
          />
        </div>
      </CardHeader>

      <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
        {data && !isLoading && (
          <div className='mb-4 flex items-center gap-2 text-sm'>
            <IconTrendingUp className='text-chart-1 size-4' />
            <span className='text-muted-foreground'>بالاترین رکورد:</span>
            <span className='font-semibold tabular-nums'>
              {formatBytesFa(data.peak_bytes)}
            </span>
            {data.peak_date && (
              <span className='text-muted-foreground'>
                ({formatDateFa(data.peak_date)})
              </span>
            )}
          </div>
        )}

        {isLoading || isFetching ? (
          <Skeleton className='h-[240px] w-full rounded-lg' />
        ) : points.length === 0 ? (
          <ReportEmptyState message='داده اوج مصرفی برای این بازه یافت نشد.' />
        ) : (
          <ChartContainer
            config={chartConfig}
            className='aspect-auto h-[240px] w-full'
          >
            <LineChart data={points}>
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
                cursor={{ strokeDasharray: '4 4' }}
                content={
                  <ChartTooltipContent
                    labelFormatter={(value) => formatDateFa(value)}
                    formatter={(value) => [formatBytesFa(Number(value)), ' حداکثر مصرف']}
                    indicator='line'
                  />
                }
              />
              <Line
                dataKey='bytes'
                type='monotone'
                stroke='var(--color-bytes)'
                strokeWidth={2}
                dot={{ r: 3, fill: 'var(--color-bytes)' }}
                activeDot={{ r: 5 }}
              />
            </LineChart>
          </ChartContainer>
        )}

        {data && data.hourly_points.length > 0 && (
          <div className='mt-6 border-t pt-4'>
            <div className='mb-3 flex items-center gap-2 text-sm'>
              <IconClock className='text-chart-2 size-4' />
              <span className='text-muted-foreground'>
                ساعت اوج مصرف:
              </span>
              {data.peak_hour !== null && data.peak_hour !== undefined ? (
                <>
                  <span className='font-semibold tabular-nums'>
                    {formatHourFa(data.peak_hour)}
                  </span>
                  <span className='text-muted-foreground'>
                    ({formatBytesFa(data.peak_hour_bytes)})
                  </span>
                </>
              ) : (
                <span className='text-muted-foreground'>—</span>
              )}
            </div>
            <ChartContainer
              config={hourlyChartConfig}
              className='aspect-auto h-[180px] w-full'
            >
              <BarChart data={data.hourly_points}>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey='hour'
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  interval={2}
                  tickFormatter={(value) => formatHourFa(value)}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  width={56}
                  tickFormatter={(value) => formatBytesFa(value)}
                />
                <ChartTooltip
                  cursor={{ fill: 'var(--muted)', opacity: 0.4 }}
                  content={
                    <ChartTooltipContent
                      labelFormatter={(value) => formatHourFa(Number(value))}
                      formatter={(value) => [
                        formatBytesFa(Number(value)),
                        ' مصرف در این ساعت',
                      ]}
                    />
                  }
                />
                <Bar
                  dataKey='bytes'
                  fill='var(--color-bytes)'
                  radius={[4, 4, 0, 0]}
                />
              </BarChart>
            </ChartContainer>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
