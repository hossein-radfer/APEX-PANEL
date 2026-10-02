import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createReseller } from '@/api/resellers.ts'

export const useCreateResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['resellers_list'] })
    },
  })
}
