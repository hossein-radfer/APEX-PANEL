import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccountStatus } from '@/api/user-manager.ts'

export const useUpdateUserManagerAccountStatusMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateUserManagerAccountStatus,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
