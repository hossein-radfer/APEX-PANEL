import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountConfig } from '@/api/user-manager.ts'

export const useUserManagerAccountConfigQuery = (
  id: number,
  options?: { enabled?: boolean }
) =>
  useQuery<Blob>({
    queryKey: ['user_manager_account_config', id],
    queryFn: () => fetchUserManagerAccountConfig(id),
    enabled: !!id && (options?.enabled ?? true),
  })
