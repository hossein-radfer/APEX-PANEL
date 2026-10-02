import { useQuery } from '@tanstack/react-query'
import { fetchUserRankingReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useUserRankingReportQuery = (
  range: ReportsRange,
  protocol: string
) =>
  useQuery({
    queryKey: ['reports_user_ranking', range, protocol],
    queryFn: () => fetchUserRankingReport(range, protocol),
  })
