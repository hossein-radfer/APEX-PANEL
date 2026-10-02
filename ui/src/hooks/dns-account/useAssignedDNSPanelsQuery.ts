import { useQuery } from '@tanstack/react-query'
import { fetchAssignedDNSPanels } from '@/api/dns-account.ts'

// Bare-ID counterpart of useAssignedDNSPanelSummariesQuery -- used by the
// admin's reseller-edit form (checkbox list against the full admin-only
// DNS panel list), same convention as useAssignedXuiPanelsQuery.
export const useAssignedDNSPanelsQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_dns_panels', resellerId],
    queryFn: () => fetchAssignedDNSPanels(resellerId as number),
    enabled: resellerId !== undefined,
  })
