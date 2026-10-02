import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccount } from '@/api/user-manager.ts'

export const useUpdateUserManagerAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateUserManagerAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
