import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerAccountsList } from '@/api/user-manager.ts'

export const useUserManagerAccountsListQuery = () =>
  useQuery({
    queryKey: ['user_manager_accounts_list'],
    queryFn: fetchUserManagerAccountsList,
  })
