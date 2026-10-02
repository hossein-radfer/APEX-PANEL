import { useQuery } from '@tanstack/react-query'
import { fetchAssignedUserManagerGroups } from '@/api/resellers.ts'

export const useAssignedUserManagerGroupsQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_user_manager_groups', resellerId],
    queryFn: () => fetchAssignedUserManagerGroups(resellerId as number),
    enabled: resellerId !== undefined,
  })
