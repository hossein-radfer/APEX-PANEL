import { useQuery } from '@tanstack/react-query'
import { fetchAssignedUserManagerGroups } from '@/api/resellers.ts'

export const useAssignedUserManagerGroupsQuery = (
  resellerId?: number,
  serverId?: number
) =>
  useQuery({
    queryKey: ['reseller_assigned_user_manager_groups', resellerId, serverId],
    queryFn: () => fetchAssignedUserManagerGroups(resellerId as number, serverId),
    enabled: resellerId !== undefined,
  })
