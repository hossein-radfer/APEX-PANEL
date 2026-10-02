import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setV2RaySaleTitle } from '@/api/v2ray.ts'

export const useSetV2RaySaleTitleMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setV2RaySaleTitle,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_sale_titles'] })
    },
  })
}
