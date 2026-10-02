import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updatePeerForReseller } from '@/api/peers.ts'

export const useUpdatePeerForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updatePeerForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['peers_list_by_reseller', variables.resellerId],
      })
    },
  })
}
