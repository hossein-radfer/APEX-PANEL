import { useQuery } from '@tanstack/react-query'
import { fetchExpiringSoonReport } from '@/api/reports.ts'

export const useExpiringSoonReportQuery = () =>
  useQuery({
    queryKey: ['reports_expiring_soon'],
    queryFn: fetchExpiringSoonReport,
  })
