import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteUserManagerProtocolClientAppFile } from '@/api/user-manager.ts'

export const useDeleteUserManagerProtocolClientAppFileMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteUserManagerProtocolClientAppFile,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_protocol_configs'],
      })
    },
  })
}
