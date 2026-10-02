import { useMutation, useQueryClient } from '@tanstack/react-query'
import { changeUserManagerAccountPassword } from '@/api/user-manager.ts'

export const useChangeUserManagerAccountPasswordMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: changeUserManagerAccountPassword,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
