import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAssignedDNSPanels } from '@/api/dns-account.ts'

export const useSetAssignedDNSPanelsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAssignedDNSPanels,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['reseller_assigned_dns_panels', variables.resellerId],
      })
      queryClient.invalidateQueries({
        queryKey: [
          'reseller_assigned_dns_panel_summaries',
          variables.resellerId,
        ],
      })
    },
  })
}
