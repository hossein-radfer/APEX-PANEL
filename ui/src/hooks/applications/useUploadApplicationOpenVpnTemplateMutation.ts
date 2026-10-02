import { useMutation, useQueryClient } from '@tanstack/react-query'
import { uploadApplicationOpenVpnTemplate } from '@/api/application.ts'

export const useUploadApplicationOpenVpnTemplateMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: uploadApplicationOpenVpnTemplate,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['application_openvpn_template_status'],
      })
    },
  })
}
