import { useMutation, useQueryClient } from '@tanstack/react-query'
import { changeUserManagerAccountPasswordForReseller } from '@/api/user-manager.ts'

export const useChangeUserManagerAccountPasswordForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: changeUserManagerAccountPasswordForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
