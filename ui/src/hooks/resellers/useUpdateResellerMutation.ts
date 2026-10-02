import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateReseller } from '@/api/resellers.ts'

export const useUpdateResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['resellers_list'] })
    },
  })
}
