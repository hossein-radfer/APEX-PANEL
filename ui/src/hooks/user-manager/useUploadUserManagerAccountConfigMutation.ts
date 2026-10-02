import { useMutation, useQueryClient } from '@tanstack/react-query'
import { uploadUserManagerAccountConfig } from '@/api/user-manager.ts'

export const useUploadUserManagerAccountConfigMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: uploadUserManagerAccountConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
