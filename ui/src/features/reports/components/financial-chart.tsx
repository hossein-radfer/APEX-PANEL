import { useMemo, useState } from 'react'
import { IconCash } from '@tabler/icons-react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import { useFinancialReportQuery } from '@/hooks/reports/useFinancialReportQuery.ts'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { ReportsFinancialBucket, ReportsRange } from '@/schema/reports.ts'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
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
import { ReportEmptyState } from '@/features/reports/components/report-empty-state.tsx'

const chartConfig = {
  charge_amount: {
    label: 'واریزی',
    color: 'var(--chart-2)',
  },
  debit_amount: {
    label: 'برداشت',
    color: 'var(--chart-5)',
  },
} satisfies ChartConfig

const BUCKET_OPTIONS: { value: ReportsFinancialBucket; label: string }[] = [
  { value: 'day', label: 'روزانه' },
  { value: 'week', label: 'هفتگی' },
  { value: 'month', label: 'ماهانه' },
]

type FinancialChartProps = {
  range: ReportsRange
}

// Report 10 -- ledger activity (charge/debit) bucketed by day/week/month,
// grouped bar chart, plus a small breakdown-by-reseller list underneath.
export function FinancialChart({ range }: FinancialChartProps) {
  const [bucket, setBucket] = useState<ReportsFinancialBucket>('day')
  const { data, isLoading, isFetching } = useFinancialReportQuery(
    range,
    bucket
  )

  const points = useMemo(() => data?.points ?? [], [data])
  const topResellers = useMemo(
    () => [...(data?.resellers ?? [])].sort((a, b) => b.total_amount - a.total_amount).slice(0, 5),
    [data]
  )

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 col-span-1 flex flex-col pt-0 duration-500 lg:col-span-2'>
      <CardHeader className='flex flex-col gap-3 space-y-0 border-b py-5 sm:flex-row sm:items-center'>
        <div className='grid flex-1 gap-1'>
          <CardTitle>
            <h2 className='text-lg font-semibold'>گزارش مالی</h2>
          </CardTitle>
          <p className='text-muted-foreground text-sm'>
            واریزی و برداشت کیف پول در بازه انتخاب‌شده
          </p>
        </div>
        <Select
          value={bucket}
          onValueChange={(value) =>
            setBucket(value as ReportsFinancialBucket)
          }
        >
          <SelectTrigger className='w-full sm:w-[140px]'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {BUCKET_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </CardHeader>

      <CardContent className='px-2 pt-4 sm:px-6 sm:pt-6'>
        {isLoading || isFetching ? (
          <Skeleton className='h-[260px] w-full rounded-lg' />
        ) : points.length === 0 ? (
          <ReportEmptyState
            icon={<IconCash className='size-10 opacity-60' />}
            message='تراکنش مالی‌ای در این بازه ثبت نشده است.'
          />
        ) : (
          <>
            <ChartContainer
              config={chartConfig}
              className='aspect-auto h-[260px] w-full'
            >
              <BarChart data={points}>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey='bucket'
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
                          (chartConfig[name as keyof typeof chartConfig]
                            ?.label ?? name),
                      ]}
                    />
                  }
                />
                <Bar
                  dataKey='charge_amount'
                  fill='var(--color-charge_amount)'
                  radius={[4, 4, 0, 0]}
                />
                <Bar
                  dataKey='debit_amount'
                  fill='var(--color-debit_amount)'
                  radius={[4, 4, 0, 0]}
                />
                <ChartLegend content={<ChartLegendContent />} />
              </BarChart>
            </ChartContainer>

            {topResellers.length > 0 && (
              <div className='mt-4 border-t pt-4'>
                <p className='text-muted-foreground mb-2 text-sm font-medium'>
                  بیشترین گردش مالی به تفکیک نماینده
                </p>
                <ul className='space-y-1.5'>
                  {topResellers.map((reseller) => (
                    <li
                      key={reseller.reseller_id}
                      className='flex items-center justify-between text-sm'
                    >
                      <span className='truncate'>
                        {reseller.reseller_name}
                      </span>
                      <span className='tabular-nums font-medium'>
                        {formatCurrencyFa(reseller.total_amount)}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
