import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteReseller } from '@/api/resellers.ts'

export const useDeleteResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['resellers_list'] })
    },
  })
}
