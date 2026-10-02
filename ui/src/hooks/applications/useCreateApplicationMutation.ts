import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createApplication } from '@/api/application.ts'

export const useCreateApplicationMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createApplication,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['applications_list'] })
      queryClient.invalidateQueries({
        queryKey: ['applications_list_by_reseller'],
      })
    },
  })
}
