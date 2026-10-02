import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteUserManagerAccountForReseller } from '@/api/user-manager.ts'

export const useDeleteUserManagerAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteUserManagerAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
