import { useQuery } from '@tanstack/react-query'
import { fetchServersList } from '@/api/servers.ts'

export const useServersListQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['servers_list'],
    queryFn: fetchServersList,
    enabled,
  })
