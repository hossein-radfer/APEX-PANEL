import { useQuery } from '@tanstack/react-query'
import { fetchDailyUsageReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useDailyUsageReportQuery = (
  range: ReportsRange,
  protocol?: string
) =>
  useQuery({
    queryKey: ['reports_daily_usage', range, protocol],
    queryFn: () => fetchDailyUsageReport(range, protocol),
  })
