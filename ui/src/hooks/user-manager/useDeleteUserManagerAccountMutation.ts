import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteUserManagerAccount } from '@/api/user-manager.ts'

export const useDeleteUserManagerAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteUserManagerAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
