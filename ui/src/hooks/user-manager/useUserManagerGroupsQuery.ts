import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerGroups } from '@/api/user-manager.ts'

export const useUserManagerGroupsQuery = (serverId?: number) =>
  useQuery({
    queryKey: ['user_manager_groups', serverId],
    queryFn: () => fetchUserManagerGroups(serverId),
  })
