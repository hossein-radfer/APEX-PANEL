import { useMutation, useQueryClient } from '@tanstack/react-query'
import { uploadUserManagerProtocolClientAppFile } from '@/api/user-manager.ts'

export const useUploadUserManagerProtocolClientAppFileMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: uploadUserManagerProtocolClientAppFile,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_protocol_configs'],
      })
    },
  })
}
