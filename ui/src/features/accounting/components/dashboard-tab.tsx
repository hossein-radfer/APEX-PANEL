import { useQuery } from '@tanstack/react-query'
import { IconCash } from '@tabler/icons-react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { fetchSummary } from '@/api/accounting.ts'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import {
  ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'

const chartConfig = {
  income_toman: {
    label: 'درآمد',
    color: 'var(--chart-2)',
  },
  cost_toman: {
    label: 'هزینه',
    color: 'var(--chart-5)',
  },
} satisfies ChartConfig

export function DashboardTab() {
  const { data, isLoading } = useQuery({
    queryKey: ['accounting_summary'],
    queryFn: () => fetchSummary(),
  })

  const profit = data?.profit_toman ?? 0

  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground text-sm font-medium'>
              مجموع درآمد (۳۰ روز اخیر)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className='h-8 w-32' />
            ) : (
              <p className='text-2xl font-bold tabular-nums text-emerald-600 dark:text-emerald-400'>
                {formatCurrencyFa(data?.total_income_toman)}
              </p>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground text-sm font-medium'>
              مجموع هزینه (۳۰ روز اخیر)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className='h-8 w-32' />
            ) : (
              <p className='text-2xl font-bold tabular-nums text-red-600 dark:text-red-400'>
                {formatCurrencyFa(data?.total_cost_toman)}
              </p>
            )}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground text-sm font-medium'>
              سود خالص (۳۰ روز اخیر)
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className='h-8 w-32' />
            ) : (
              <p
                className={`text-2xl font-bold tabular-nums ${profit >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}
              >
                {formatCurrencyFa(profit)}
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>روند درآمد و هزینه</CardTitle>
        </CardHeader>
        <CardContent className='px-2 pt-4 sm:px-6'>
          {isLoading ? (
            <Skeleton className='h-[260px] w-full rounded-lg' />
          ) : !data || data.series.length === 0 ? (
            <EmptyState
              icon={<IconCash className='size-10 opacity-60' />}
              message='در این بازه هیچ تراکنشی ثبت نشده است.'
            />
          ) : (
            <ChartContainer config={chartConfig} className='aspect-auto h-[260px] w-full'>
              <BarChart data={data.series}>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey='period'
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={24}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  width={70}
                  tickFormatter={(value) => formatCurrencyFa(value)}
                />
                <ChartTooltip
                  cursor={{ fill: 'var(--muted)', opacity: 0.4 }}
                  content={
                    <ChartTooltipContent
                      formatter={(value, name) => [
                        formatCurrencyFa(Number(value)),
                        ' ' +
                          (chartConfig[name as keyof typeof chartConfig]?.label ?? name),
                      ]}
                    />
                  }
                />
                <Bar dataKey='income_toman' fill='var(--color-income_toman)' radius={[4, 4, 0, 0]} />
                <Bar dataKey='cost_toman' fill='var(--color-cost_toman)' radius={[4, 4, 0, 0]} />
                <ChartLegend content={<ChartLegendContent />} />
              </BarChart>
            </ChartContainer>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
