import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDNSAccount } from '@/api/dns-account.ts'

export const useUpdateDNSAccountMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateDNSAccount,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_accounts_list'] })
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller'],
      })
    },
  })
}
