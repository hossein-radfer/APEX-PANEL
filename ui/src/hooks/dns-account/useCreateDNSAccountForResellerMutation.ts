import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDNSAccountForReseller } from '@/api/dns-account.ts'

export const useCreateDNSAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createDNSAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
