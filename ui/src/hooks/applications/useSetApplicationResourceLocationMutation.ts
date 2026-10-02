import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setApplicationResourceLocation } from '@/api/application.ts'

export const useSetApplicationResourceLocationMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setApplicationResourceLocation,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['application_resource_locations'],
      })
    },
  })
}
