import { useMutation, useQueryClient } from '@tanstack/react-query'
import { uploadUserManagerAccountConfigForReseller } from '@/api/user-manager.ts'

export const useUploadUserManagerAccountConfigForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: uploadUserManagerAccountConfigForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
