import { useQuery } from '@tanstack/react-query'
import { fetchServerEndpoints } from '@/api/servers.ts'

export const useServerEndpointsQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['server_endpoints'],
    queryFn: fetchServerEndpoints,
    enabled,
  })
