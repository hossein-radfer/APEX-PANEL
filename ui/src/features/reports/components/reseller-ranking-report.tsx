import { useResellerRankingReportQuery } from '@/hooks/reports/useResellerRankingReportQuery.ts'
import { ReportsRange } from '@/schema/reports.ts'
import { RankingBarChart } from '@/features/reports/components/ranking-bar-chart.tsx'

// Report 3 -- resellers ranked by total usage across all their
// customers/protocols in the selected range.
export function ResellerRankingReport({ range }: { range: ReportsRange }) {
  const { data, isLoading } = useResellerRankingReportQuery(range)

  return (
    <RankingBarChart
      title='رتبه‌بندی نمایندگان بر اساس مصرف'
      description='پرمصرف‌ترین نمایندگان در بازه انتخاب‌شده'
      rows={data?.rows}
      isLoading={isLoading}
      emptyMessage='هیچ نماینده‌ای در این بازه مصرفی نداشته است.'
    />
  )
}
