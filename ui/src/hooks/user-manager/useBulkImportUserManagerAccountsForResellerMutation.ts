import { useMutation, useQueryClient } from '@tanstack/react-query'
import { bulkImportUserManagerAccountsForReseller } from '@/api/user-manager.ts'

export const useBulkImportUserManagerAccountsForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: bulkImportUserManagerAccountsForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
