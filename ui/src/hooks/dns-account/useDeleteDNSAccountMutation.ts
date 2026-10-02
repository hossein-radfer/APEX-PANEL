import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteDNSAccount } from '@/api/dns-account.ts'

export const useDeleteDNSAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteDNSAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller'],
      })
    },
  })
}
