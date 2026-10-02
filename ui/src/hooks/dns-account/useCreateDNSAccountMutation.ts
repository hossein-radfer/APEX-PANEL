import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDNSAccount } from '@/api/dns-account.ts'

export const useCreateDNSAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createDNSAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller'],
      })
    },
  })
}
