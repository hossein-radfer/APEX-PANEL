import { useQuery } from '@tanstack/react-query'
import { fetchV2RaySelfSummary } from '@/api/v2ray.ts'

export const useV2RaySelfSummaryQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['v2ray_self_summary'],
    queryFn: fetchV2RaySelfSummary,
    enabled,
    refetchInterval: 30_000,
  })
