import { useMutation } from '@tanstack/react-query'
import { testSavedDNSPanelForReseller } from '@/api/dns-panel.ts'

// Reseller-safe counterpart to useTestSavedDNSPanelMutation -- see
// testSavedDNSPanelForReseller's own doc comment for the confirmed,
// reported 403 bug this fixes. No query invalidation needed here (unlike
// the admin mutation): GET /dns-panel itself is still admin-only, so a
// reseller session never holds that list in its cache to begin with.
export const useTestSavedDNSPanelForResellerMutation = () =>
  useMutation({
    mutationFn: ({ resellerId, id }: { resellerId: number; id: number }) =>
      testSavedDNSPanelForReseller(resellerId, id),
  })
