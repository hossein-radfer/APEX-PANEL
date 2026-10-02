import { useMemo, useState } from 'react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { formatBytesFa, protocolLabelFa } from '@/features/reports/lib/format.ts'
import { ReportsRankingRow } from '@/schema/reports.ts'
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
  bytes: {
    label: 'حجم مصرف',
    color: 'var(--chart-1)',
  },
} satisfies ChartConfig

type RankingBarChartProps = {
  title: string
  description: string
  rows: ReportsRankingRow[] | undefined
  isLoading: boolean
  valueLabel?: string
  showPackageCount?: boolean
  initialLimit?: number
  emptyMessage?: string
}

// Shared horizontal bar-chart ranking view for reports 3 (reseller
// usage ranking), 4 (user usage ranking) and 5 (reseller activity ranking)
// -- sorted descending, top-N with a "show more" toggle for long lists.
export function RankingBarChart({
  title,
  description,
  rows,
  isLoading,
  showPackageCount,
  initialLimit = 8,
  emptyMessage = 'داده‌ای برای این بازه ثبت نشده است.',
}: RankingBarChartProps) {
  const [showAll, setShowAll] = useState(false)

  const sorted = useMemo(
    () => [...(rows ?? [])].sort((a, b) => b.bytes - a.bytes),
    [rows]
  )
  const visible = showAll ? sorted : sorted.slice(0, initialLimit)
  const hasMore = sorted.length > initialLimit

  const chartHeight = Math.max(160, visible.length * 42)

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 flex flex-col pt-0 duration-500'>
      <CardHeader className='space-y-0 border-b py-5'>
        <CardTitle>
          <h2 className='text-lg font-semibold'>{title}</h2>
        </CardTitle>
        <p className='text-muted-foreground text-sm'>{description}</p>
      </CardHeader>

      <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
        {isLoading ? (
          <Skeleton className='h-[220px] w-full rounded-lg' />
        ) : sorted.length === 0 ? (
          <ReportEmptyState message={emptyMessage} />
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
                  tickFormatter={(value) => formatBytesFa(value)}
                />
                <YAxis
                  type='category'
                  dataKey='name'
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
                        const row = item?.payload as ReportsRankingRow
                        return [
                          <div
                            key='content'
                            className='flex w-full flex-col gap-0.5'
                          >
                            <span className='text-foreground font-medium'>
                              {row?.name}
                            </span>
                            <span className='text-muted-foreground'>
                              {formatBytesFa(Number(value))}
                              {row?.protocol
                                ? ` — ${protocolLabelFa(row.protocol)}`
                                : ''}
                              {showPackageCount && row?.package_count
                                ? ` — ${row.package_count} بسته`
                                : ''}
                            </span>
                          </div>,
                          '',
                        ]
                      }}
                    />
                  }
                />
                <Bar
                  dataKey='bytes'
                  fill='var(--color-bytes)'
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
