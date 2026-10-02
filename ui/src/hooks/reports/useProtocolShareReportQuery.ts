import { useQuery } from '@tanstack/react-query'
import { fetchProtocolShareReport } from '@/api/reports.ts'
import { ReportsRange } from '@/schema/reports.ts'

export const useProtocolShareReportQuery = (range: ReportsRange) =>
  useQuery({
    queryKey: ['reports_protocol_share', range],
    queryFn: () => fetchProtocolShareReport(range),
  })
