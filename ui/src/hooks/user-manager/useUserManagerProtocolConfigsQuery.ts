import { useQuery } from '@tanstack/react-query'
import { fetchUserManagerProtocolConfigs } from '@/api/user-manager.ts'

export const useUserManagerProtocolConfigsQuery = (serverId?: number) =>
  useQuery({
    queryKey: ['user_manager_protocol_configs', serverId],
    queryFn: () => fetchUserManagerProtocolConfigs(serverId),
  })
