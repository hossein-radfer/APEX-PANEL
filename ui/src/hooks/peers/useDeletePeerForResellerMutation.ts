import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deletePeerForReseller } from '@/api/peers.ts'

export const useDeletePeerForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deletePeerForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['peers_list_by_reseller', variables.resellerId],
      })
    },
  })
}
