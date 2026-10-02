import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerProfiles } from '@/api/user-manager.ts'

export const useUserManagerProfilesQuery = () =>
  useQuery({
    queryKey: ['user_manager_profiles'],
    queryFn: fetchUserManagerProfiles,
  })
