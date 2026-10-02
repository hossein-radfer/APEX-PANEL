import { useMutation, useQueryClient } from '@tanstack/react-query'
import { registerDNSIP } from '@/api/dns-account.ts'

// Registers the caller's own current IP against a shared DNS account --
// takes no parameters beyond the share uuid, see registerDNSIP's own doc
// comment. Refetches the share details afterward regardless of ok/false so
// the status card's registrations-used-today/current-IP figures update
// immediately.
export const useRegisterDNSIPMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: registerDNSIP,
    onSuccess: (_data, uuid) => {
      queryClient.invalidateQueries({
        queryKey: ['dns_account_share_details', uuid],
      })
    },
  })
}
