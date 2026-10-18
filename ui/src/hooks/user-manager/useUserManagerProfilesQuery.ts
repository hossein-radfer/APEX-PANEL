import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerProfiles } from '@/api/user-manager.ts'

export const useUserManagerProfilesQuery = (serverId?: number) =>
  useQuery({
    queryKey: ['user_manager_profiles', serverId],
    queryFn: () => fetchUserManagerProfiles(serverId),
  })
