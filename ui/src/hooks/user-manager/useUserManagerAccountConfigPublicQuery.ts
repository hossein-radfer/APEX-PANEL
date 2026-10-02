import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountConfigPublic } from '@/api/user-manager.ts'

export const useUserManagerAccountConfigPublicQuery = (
  uuid: string | undefined,
  options?: { enabled?: boolean }
) =>
  useQuery<Blob>({
    queryKey: ['user_manager_account_config_public', uuid],
    queryFn: () => fetchUserManagerAccountConfigPublic(uuid as string),
    enabled: !!uuid && (options?.enabled ?? true),
  })
