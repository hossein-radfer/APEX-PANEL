import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createPeerForReseller } from '@/api/peers.ts'

export const useCreatePeerForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createPeerForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['peers_list_by_reseller', variables.resellerId],
      })
    },
  })
}
