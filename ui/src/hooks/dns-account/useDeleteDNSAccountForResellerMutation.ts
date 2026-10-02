import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteDNSAccountForReseller } from '@/api/dns-account.ts'

export const useDeleteDNSAccountForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteDNSAccountForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_accounts_list_by_reseller', variables.resellerId],
      })
    },
  })
}
