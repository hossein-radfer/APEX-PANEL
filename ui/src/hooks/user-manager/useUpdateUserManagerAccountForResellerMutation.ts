import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccountForReseller } from '@/api/user-manager.ts'

export const useUpdateUserManagerAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateUserManagerAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
