import { useQuery } from '@tanstack/react-query'
import { fetchAssignedUserManagerProfiles } from '@/api/resellers.ts'

export const useAssignedUserManagerProfilesQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_user_manager_profiles', resellerId],
    queryFn: () => fetchAssignedUserManagerProfiles(resellerId as number),
    enabled: resellerId !== undefined,
  })
