import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountsByReseller } from '@/api/user-manager.ts'

export const useUserManagerAccountsByResellerQuery = (
  resellerId: number | undefined
) =>
  useQuery({
    queryKey: ['user_manager_accounts_list_by_reseller', resellerId],
    queryFn: () => fetchUserManagerAccountsByReseller(resellerId as number),
    enabled: !!resellerId,
  })
