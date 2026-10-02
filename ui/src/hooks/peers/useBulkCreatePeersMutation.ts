import { useMutation, useQueryClient } from '@tanstack/react-query'
import { bulkCreatePeers } from '@/api/peers.ts'

export const useBulkCreatePeersMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: bulkCreatePeers,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['peers_list'] })
      queryClient.invalidateQueries({ queryKey: ['peers_list_by_reseller'] })
    },
  })
}
