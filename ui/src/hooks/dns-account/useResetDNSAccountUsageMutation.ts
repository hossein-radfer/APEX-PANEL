import { useMutation, useQueryClient } from '@tanstack/react-query'
import { resetDNSAccountUsage } from '@/api/dns-account.ts'

export const useResetDNSAccountUsageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: resetDNSAccountUsage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller'],
      })
    },
  })
}
