import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteApplication } from '@/api/application.ts'

export const useDeleteApplicationMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteApplication,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications_list'] })
      queryClient.invalidateQueries({
        queryKey: ['applications_list_by_reseller'],
      })
    },
  })
}
