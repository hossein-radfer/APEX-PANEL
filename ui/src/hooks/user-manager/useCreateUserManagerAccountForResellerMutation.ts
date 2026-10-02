import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createUserManagerAccountForReseller } from '@/api/user-manager.ts'

export const useCreateUserManagerAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createUserManagerAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
