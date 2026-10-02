import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishAppVersion } from '@/api/application.ts'

export const usePublishAppVersionMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: publishAppVersion,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app_versions'] })
    },
  })
}
