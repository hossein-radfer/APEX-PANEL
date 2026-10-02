import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteDNSPanel } from '@/api/dns-panel.ts'

export const useDeleteDNSPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteDNSPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_panels_list'] })
    },
  })
}
