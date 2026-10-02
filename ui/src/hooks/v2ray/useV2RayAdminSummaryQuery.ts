import { useQuery } from '@tanstack/react-query'
import { fetchV2RayAdminSummary } from '@/api/v2ray.ts'

export const useV2RayAdminSummaryQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['v2ray_admin_summary'],
    queryFn: fetchV2RayAdminSummary,
    enabled,
    refetchInterval: 30_000,
  })
