import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updatePeerStatusForReseller } from '@/api/peers.ts'

export const useUpdatePeerStatusForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updatePeerStatusForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['peers_list_by_reseller', variables.resellerId],
      })
    },
  })
}
