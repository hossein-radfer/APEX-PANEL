import { useMemo, useState } from 'react'
import { IconMapPin } from '@tabler/icons-react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { usePopularLocationsReportQuery } from '@/hooks/reports/usePopularLocationsReportQuery.ts'
import { formatBytesFa } from '@/features/reports/lib/format.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { ReportEmptyState } from '@/features/reports/components/report-empty-state.tsx'

const chartConfig = {
  sales_count: {
    label: 'تعداد فروش',
    color: 'var(--chart-1)',
  },
} satisfies ChartConfig

type PopularLocationsChartProps = {
  range: ReportsRange
}

// Report 12 -- most popular locations (panels) by sales count. Horizontal
// bar chart (mirrors RankingBarChart's own layout/top-N/show-more pattern)
// sorted descending -- converted from a vertical bar chart per admin
// feedback that this report's chart type didn't fit the panel.
export function PopularLocationsChart({ range }: PopularLocationsChartProps) {
  const { data, isLoading, isFetching } = usePopularLocationsReportQuery(
    range
  )
  const [showAll, setShowAll] = useState(false)
  const initialLimit = 8

  const sorted = useMemo(
    () =>
      [...(data?.rows ?? [])].sort((a, b) => b.sales_count - a.sales_count),
    [data]
  )
  const visible = showAll ? sorted : sorted.slice(0, initialLimit)
  const hasMore = sorted.length > initialLimit
  const chartHeight = Math.max(160, visible.length * 42)

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 flex flex-col pt-0 duration-500'>
      <CardHeader className='space-y-0 border-b py-5'>
        <CardTitle>
          <h2 className='text-lg font-semibold'>محبوب‌ترین موقعیت‌ها</h2>
        </CardTitle>
        <p className='text-muted-foreground text-sm'>
          پرفروش‌ترین سرورها بر اساس تعداد فروش و حجم مصرف
        </p>
      </CardHeader>

      <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
        {isLoading || isFetching ? (
          <Skeleton className='h-[240px] w-full rounded-lg' />
        ) : sorted.length === 0 ? (
          <ReportEmptyState
            icon={<IconMapPin className='size-10 opacity-60' />}
            message='داده فروشی برای موقعیت‌ها ثبت نشده است.'
          />
        ) : (
          <>
            <ChartContainer
              config={chartConfig}
              className='w-full'
              style={{ height: chartHeight }}
            >
              <BarChart
                data={visible}
                layout='vertical'
                margin={{ left: 8, right: 24 }}
              >
                <CartesianGrid horizontal={false} />
                <XAxis
                  type='number'
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(value) =>
                    Number(value).toLocaleString('fa-IR')
                  }
                />
                <YAxis
                  type='category'
                  dataKey='panel_name'
                  tickLine={false}
                  axisLine={false}
                  width={110}
                  tick={{ fontSize: 12 }}
                  tickFormatter={(value: string) =>
                    value.length > 14 ? `${value.slice(0, 14)}…` : value
                  }
                />
                <ChartTooltip
                  cursor={{ fill: 'var(--muted)', opacity: 0.4 }}
                  content={
                    <ChartTooltipContent
                      hideLabel
                      formatter={(value, _name, item) => {
                        const row = item?.payload
                        return [
                          <div key='c' className='flex flex-col gap-0.5'>
                            <span className='text-foreground font-medium'>
                              {row?.panel_name}
                            </span>
                            <span className='text-muted-foreground'>
                              {Number(value).toLocaleString('fa-IR')} فروش —{' '}
                              {formatBytesFa(row?.bytes)}
                            </span>
                          </div>,
                          '',
                        ]
                      }}
                    />
                  }
                />
                <Bar
                  dataKey='sales_count'
                  fill='var(--color-sales_count)'
                  radius={[0, 4, 4, 0]}
                />
              </BarChart>
            </ChartContainer>

            {hasMore && (
              <div className='mt-3 flex justify-center'>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => setShowAll((prev) => !prev)}
                >
                  {showAll
                    ? 'نمایش کمتر'
                    : `نمایش همه (${sorted.length.toLocaleString('fa-IR')})`}
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
