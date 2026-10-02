import { useQuery } from '@tanstack/react-query'
import { fetchResellerRankingReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useResellerRankingReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_reseller_ranking', range],
    queryFn: () => fetchResellerRankingReport(range),
  })
