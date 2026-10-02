import { useMutation, useQueryClient } from '@tanstack/react-query'
import { uploadUserManagerProtocolCertificateFile } from '@/api/user-manager.ts'

export const useUploadUserManagerProtocolCertificateFileMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: uploadUserManagerProtocolCertificateFile,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_protocol_configs'],
      })
    },
  })
}
