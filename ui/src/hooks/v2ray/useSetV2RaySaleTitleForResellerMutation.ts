import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setV2RaySaleTitleForReseller } from '@/api/v2ray.ts'

export const useSetV2RaySaleTitleForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setV2RaySaleTitleForReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_sale_titles'] })
    },
  })
}
