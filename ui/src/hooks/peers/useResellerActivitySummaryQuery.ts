import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { fetchResellerActivitySummary } from '@/api/peers.ts'

export const useResellerActivitySummaryQuery = (
  page: number = 1,
  pageSize: number = 20,
  enabled: boolean = true
) =>
  useQuery({
    queryKey: ['reseller_activity_summary', page, pageSize],
    queryFn: () => fetchResellerActivitySummary(page, pageSize),
    enabled,
    refetchInterval: 30_000,
    placeholderData: keepPreviousData,
  })
