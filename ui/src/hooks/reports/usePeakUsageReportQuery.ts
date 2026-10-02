import { useQuery } from '@tanstack/react-query'
import { fetchPeakUsageReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const usePeakUsageReportQuery = (
  range: ReportsRange,
  entity?: string
) =>
  useQuery({
    queryKey: ['reports_peak_usage', range, entity],
    queryFn: () => fetchPeakUsageReport(range, entity),
  })
