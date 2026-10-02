import { useQuery } from '@tanstack/react-query'
import { fetchPopularLocationsReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const usePopularLocationsReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_popular_locations', range],
    queryFn: () => fetchPopularLocationsReport(range),
  })
