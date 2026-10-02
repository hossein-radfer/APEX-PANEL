import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerSelfSummary } from '@/api/user-manager.ts'

export const useUserManagerSelfSummaryQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['user_manager_self_summary'],
    queryFn: fetchUserManagerSelfSummary,
    enabled,
    refetchInterval: 30_000,
  })
