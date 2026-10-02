import { useMutation, useQueryClient } from '@tanstack/react-query'
import { bulkImportUserManagerAccounts } from '@/api/user-manager.ts'

export const useBulkImportUserManagerAccountsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: bulkImportUserManagerAccounts,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
    },
  })
}
