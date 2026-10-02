import { useMutation, useQueryClient } from '@tanstack/react-query'
import { testSavedDNSPanel } from '@/api/dns-panel.ts'

export const useTestSavedDNSPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: testSavedDNSPanel,
    onSuccess: () => {
      // A test call also refreshes the panel's stored status/last_error on
      // the backend, so the list needs to reflect that.
      queryClient.invalidateQueries({ queryKey: ['dns_panels_list'] })
    },
  })
}
