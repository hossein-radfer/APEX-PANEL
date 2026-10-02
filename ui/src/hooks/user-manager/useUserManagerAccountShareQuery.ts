import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountShareStatus } from '@/api/user-manager.ts'

export function useUserManagerAccountShareQuery(
  id: number,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: ['user_manager_account_share', id],
    queryFn: () => fetchUserManagerAccountShareStatus(id),
    enabled: !!id && (options?.enabled ?? true),
  })
}
