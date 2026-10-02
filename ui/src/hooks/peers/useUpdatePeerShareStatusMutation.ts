import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updatePeerShareStatus } from '@/api/peers.ts'

export const useUpdatePeerShareStatusMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, number>({
    mutationFn: updatePeerShareStatus,
    onSuccess: (_data, peerId) => {
      queryClient.invalidateQueries({ queryKey: ['peer_share', peerId] })
    },
  })
}
