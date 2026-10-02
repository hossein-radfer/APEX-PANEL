import { useMutation, useQueryClient } from '@tanstack/react-query'
import { bulkCreateUserManagerAccounts } from '@/api/user-manager.ts'

export const useBulkCreateUserManagerAccountsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: bulkCreateUserManagerAccounts,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
