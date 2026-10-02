import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDNSAccountShareExpire } from '@/api/dns-account.ts'
import { UpdateDNSAccountShareExpireRequest } from '@/schema/dns-account.ts'

export const useUpdateDNSAccountShareExpireMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, UpdateDNSAccountShareExpireRequest>({
    mutationFn: updateDNSAccountShareExpire,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_account_share', variables.id],
      })
    },
  })
}
