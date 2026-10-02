import { useResellerActivityReportQuery } from '@/hooks/reports/useResellerActivityReportQuery.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { RankingBarChart } from '@/features/reports/components/ranking-bar-chart.tsx'

// Report 5 -- resellers ranked by activity (usage + package_count) in the
// selected range.
export function ResellerActivityReport({ range }: { range: ReportsRange }) {
  const { data, isLoading } = useResellerActivityReportQuery(range)

  return (
    <RankingBarChart
      title='فعالیت نمایندگان'
      description='فعال‌ترین نمایندگان بر اساس مصرف و تعداد بسته‌ها'
      rows={data?.rows}
      isLoading={isLoading}
      showPackageCount
      emptyMessage='هیچ فعالیتی برای نمایندگان در این بازه ثبت نشده است.'
    />
  )
}
