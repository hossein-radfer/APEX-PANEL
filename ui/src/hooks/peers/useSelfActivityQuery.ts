import { useQuery } from '@tanstack/react-query'
import { fetchSelfActivity } from '@/api/peers.ts'

export const useSelfActivityQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['self_activity'],
    queryFn: fetchSelfActivity,
    enabled,
    refetchInterval: 30_000,
  })
