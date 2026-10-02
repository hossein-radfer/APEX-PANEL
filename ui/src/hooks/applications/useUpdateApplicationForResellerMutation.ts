import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateApplicationForReseller } from '@/api/application.ts'

export const useUpdateApplicationForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateApplicationForReseller,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications_list'] })
      queryClient.invalidateQueries({
        queryKey: ['applications_list_by_reseller'],
      })
    },
  })
}
