import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createApplicationForReseller } from '@/api/application.ts'

export const useCreateApplicationForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createApplicationForReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications_list'] })
      queryClient.invalidateQueries({
        queryKey: ['applications_list_by_reseller'],
      })
    },
  })
}
