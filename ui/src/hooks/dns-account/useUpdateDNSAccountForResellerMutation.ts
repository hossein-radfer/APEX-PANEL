import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDNSAccountForReseller } from '@/api/dns-account.ts'

export const useUpdateDNSAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateDNSAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
