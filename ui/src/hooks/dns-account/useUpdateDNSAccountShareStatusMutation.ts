import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDNSAccountShareStatus } from '@/api/dns-account.ts'

export const useUpdateDNSAccountShareStatusMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, number>({
    mutationFn: updateDNSAccountShareStatus,
    onSuccess: (_data, accountId) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_account_share', accountId],
      })
    },
  })
}
