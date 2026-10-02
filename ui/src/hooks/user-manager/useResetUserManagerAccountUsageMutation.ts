import { useMutation, useQueryClient } from '@tanstack/react-query'
import { resetUserManagerAccountUsage } from '@/api/user-manager.ts'

export const useResetUserManagerAccountUsageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: resetUserManagerAccountUsage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user_manager_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['user_manager_accounts_list_by_reseller'],
      })
    },
  })
}
