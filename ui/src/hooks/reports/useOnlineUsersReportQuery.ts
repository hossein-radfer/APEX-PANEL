import { useQuery } from '@tanstack/react-query'
import { fetchOnlineUsersReport } from '@/api/reports.ts'

// Live data -- refetches every 15s while the reports page is open, no
// range param (always "right now").
export const useOnlineUsersReportQuery = () =>
  useQuery({
    queryKey: ['reports_online_users'],
    queryFn: fetchOnlineUsersReport,
    refetchInterval: 15000,
  })
