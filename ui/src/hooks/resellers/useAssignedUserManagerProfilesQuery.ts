import { useQuery } from '@tanstack/react-query'
import { fetchAssignedUserManagerProfiles } from '@/api/resellers.ts'

export const useAssignedUserManagerProfilesQuery = (
  resellerId?: number,
  serverId?: number
) =>
  useQuery({
    queryKey: ['reseller_assigned_user_manager_profiles', resellerId, serverId],
    queryFn: () => fetchAssignedUserManagerProfiles(resellerId as number, serverId),
    enabled: resellerId !== undefined,
  })
