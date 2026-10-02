import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerProtocolConfigs } from '@/api/user-manager.ts'

export const useUserManagerProtocolConfigsQuery = () =>
  useQuery({
    queryKey: ['user_manager_protocol_configs'],
    queryFn: fetchUserManagerProtocolConfigs,
  })
