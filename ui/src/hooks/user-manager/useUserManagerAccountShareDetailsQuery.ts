import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountShareDetails } from '@/api/user-manager.ts'

export const useUserManagerAccountShareDetailsQuery = (
  uuid: string | undefined
) =>
  useQuery({
    queryKey: ['user_manager_account_share_details', uuid],
    queryFn: () => fetchUserManagerAccountShareDetails(uuid as string),
    enabled: !!uuid,
  })
