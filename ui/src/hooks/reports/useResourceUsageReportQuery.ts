import { useQuery } from '@tanstack/react-query'
import { fetchResourceUsageReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useResourceUsageReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_resource_usage', range],
    queryFn: () => fetchResourceUsageReport(range),
  })
