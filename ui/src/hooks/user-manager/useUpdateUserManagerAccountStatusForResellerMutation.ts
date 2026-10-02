import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccountStatusForReseller } from '@/api/user-manager.ts'

export const useUpdateUserManagerAccountStatusForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateUserManagerAccountStatusForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
