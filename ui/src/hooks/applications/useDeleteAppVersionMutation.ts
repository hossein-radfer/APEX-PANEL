import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteAppVersion } from '@/api/application.ts'

export const useDeleteAppVersionMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteAppVersion,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app_versions'] })
    },
  })
}
