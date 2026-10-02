import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteUserManagerProtocolCertificateFile } from '@/api/user-manager.ts'

export const useDeleteUserManagerProtocolCertificateFileMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteUserManagerProtocolCertificateFile,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_protocol_configs'],
      })
    },
  })
}
