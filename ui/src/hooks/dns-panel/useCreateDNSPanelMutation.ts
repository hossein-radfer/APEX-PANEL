import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createDNSPanel } from '@/api/dns-panel.ts'

export const useCreateDNSPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createDNSPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns_panels_list'] })
    },
  })
}
