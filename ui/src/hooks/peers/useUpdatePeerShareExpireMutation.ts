import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updatePeerShareExpire } from '@/api/peers.ts'
import { UpdatePeerShareExpireRequest } from '@/schema/peers.ts'

export const useUpdatePeerShareExpireMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, UpdatePeerShareExpireRequest>({
    mutationFn: updatePeerShareExpire,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['peer_share', variables.id] })
    },
  })
}
