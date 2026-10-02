import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateApplication } from '@/api/application.ts'

export const useUpdateApplicationMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateApplication,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications_list'] })
      queryClient.invalidateQueries({
        queryKey: ['applications_list_by_reseller'],
      })
    },
  })
}
