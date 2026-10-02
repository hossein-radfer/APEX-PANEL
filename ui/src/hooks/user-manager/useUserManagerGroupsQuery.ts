import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerGroups } from '@/api/user-manager.ts'

export const useUserManagerGroupsQuery = () =>
  useQuery({
    queryKey: ['user_manager_groups'],
    queryFn: fetchUserManagerGroups,
  })
