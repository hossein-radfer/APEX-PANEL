import { useQuery } from '@tanstack/react-query'
import { fetchFinancialReport } from '@/api/reports.ts'
import { ReportsFinancialBucket, ReportsRange } from '@/schema/reports.ts'

export const useFinancialReportQuery = (
  range: ReportsRange,
  bucket: ReportsFinancialBucket = 'day'
) =>
  useQuery({
    queryKey: ['reports_financial', range, bucket],
    queryFn: () => fetchFinancialReport(range, bucket),
  })
