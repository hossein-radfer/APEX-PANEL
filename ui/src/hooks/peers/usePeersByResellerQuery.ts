import { useQuery } from '@tanstack/react-query'
import { fetchPeersByReseller } from '@/api/peers.ts'

export const usePeersByResellerQuery = (resellerId: number | null) =>
  useQuery({
    queryKey: ['peers_list_by_reseller', resellerId],
    queryFn: () => fetchPeersByReseller(resellerId!),
    enabled: !!resellerId,
  })
