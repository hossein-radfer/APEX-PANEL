import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateDNSPanel } from '@/api/dns-panel.ts'

export const useUpdateDNSPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateDNSPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_panels_list'] })
    },
  })
}
