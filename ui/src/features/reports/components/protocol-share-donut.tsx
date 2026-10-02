import { useMemo } from 'react'
import { IconChartDonut } from '@tabler/icons-react'
import { Cell, Label, Pie, PieChart } from 'recharts'
import { useProtocolShareReportQuery } from '@/hooks/reports/useProtocolShareReportQuery.ts'
import { formatBytesFa } from '@/features/reports/lib/format.ts'
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
} satisfies ChartConfig

type ProtocolShareDonutProps = {
  range: ReportsRange
}

// Report 7 -- donut chart of total-usage share by protocol, with the
// grand total rendered as a center label (a common donut-chart pattern).
export function ProtocolShareDonut({ range }: ProtocolShareDonutProps) {
  const { data, isLoading, isFetching } = useProtocolShareReportQuery(range)

  const chartData = useMemo(() => {
    if (!data) return []
    return (
      [
        { key: 'wireguard', value: data.wireguard_bytes },
        { key: 'user_manager', value: data.user_manager_bytes },
        { key: 'v2ray', value: data.v2ray_bytes },
      ] as const
    ).filter((entry) => entry.value > 0)
  }, [data])

  const total = useMemo(
    () => chartData.reduce((sum, entry) => sum + entry.value, 0),
    [chartData]
  )

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 flex flex-col pt-0 duration-500'>
      <CardHeader className='space-y-0 border-b py-5'>
        <CardTitle>
          <h2 className='text-lg font-semibold'>سهم پروتکل‌ها از مصرف</h2>
        </CardTitle>
        <p className='text-muted-foreground text-sm'>
          توزیع حجم مصرف بین وایرگارد، یوزر منیجر و V2Ray
        </p>
      </CardHeader>

      <CardContent className='flex-1 px-2 pt-4 sm:px-6 sm:pt-6'>
        {isLoading || isFetching ? (
          <Skeleton className='mx-auto h-[260px] w-[260px] rounded-full' />
        ) : chartData.length === 0 ? (
          <ReportEmptyState
            icon={<IconChartDonut className='size-10 opacity-60' />}
            message='مصرفی در این بازه ثبت نشده است.'
          />
        ) : (
          <ChartContainer
            config={chartConfig}
            className='mx-auto aspect-square h-[260px]'
          >
            <PieChart>
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    hideLabel
                    formatter={(value, name) => [
                      formatBytesFa(Number(value)),
                      ' ' +
                        (chartConfig[name as keyof typeof chartConfig]
                          ?.label ?? name),
                    ]}
                  />
                }
              />
              <Pie
                data={chartData}
                dataKey='value'
                nameKey='key'
                innerRadius={70}
                outerRadius={100}
                strokeWidth={2}
              >
                {chartData.map((entry) => (
                  <Cell
                    key={entry.key}
                    fill={`var(--color-${entry.key})`}
                  />
                ))}
                <Label
                  content={({ viewBox }) => {
                    if (!viewBox || !('cx' in viewBox)) return null
                    return (
                      <text
                        x={viewBox.cx}
                        y={viewBox.cy}
                        textAnchor='middle'
                        dominantBaseline='middle'
                      >
                        <tspan
                          x={viewBox.cx}
                          y={viewBox.cy}
                          className='fill-foreground text-lg font-bold'
                        >
                          {formatBytesFa(total)}
                        </tspan>
                        <tspan
                          x={viewBox.cx}
                          y={(viewBox.cy ?? 0) + 20}
                          className='fill-muted-foreground text-xs'
                        >
                          مجموع مصرف
                        </tspan>
                      </text>
                    )
                  }}
                />
              </Pie>
              <ChartLegend content={<ChartLegendContent />} />
            </PieChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}
