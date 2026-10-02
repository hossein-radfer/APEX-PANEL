import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createUserManagerAccount } from '@/api/user-manager.ts'

export const useCreateUserManagerAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createUserManagerAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
