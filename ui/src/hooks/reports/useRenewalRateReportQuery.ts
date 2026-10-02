import { useQuery } from '@tanstack/react-query'
import { fetchRenewalRateReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useRenewalRateReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_renewal_rate', range],
    queryFn: () => fetchRenewalRateReport(range),
  })
