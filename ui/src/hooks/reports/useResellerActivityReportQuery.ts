import { useQuery } from '@tanstack/react-query'
import { fetchResellerActivityReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useResellerActivityReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_reseller_activity', range],
    queryFn: () => fetchResellerActivityReport(range),
  })
