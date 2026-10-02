import { useQuery } from '@tanstack/react-query'
import { fetchAssignedDNSPanelSummaries } from '@/api/dns-account.ts'

export const useAssignedDNSPanelSummariesQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_dns_panel_summaries', resellerId],
    queryFn: () => fetchAssignedDNSPanelSummaries(resellerId as number),
    enabled: resellerId !== undefined,
  })
