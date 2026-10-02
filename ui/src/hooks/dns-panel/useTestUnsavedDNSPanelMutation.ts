import { useMutation } from '@tanstack/react-query'
import { testUnsavedDNSPanel } from '@/api/dns-panel.ts'

export const useTestUnsavedDNSPanelMutation = () =>
  useMutation({
    mutationFn: testUnsavedDNSPanel,
  })
