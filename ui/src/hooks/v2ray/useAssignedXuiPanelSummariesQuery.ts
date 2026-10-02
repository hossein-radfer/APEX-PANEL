import { useQuery } from '@tanstack/react-query'
import { fetchAssignedXuiPanelSummaries } from '@/api/v2ray.ts'

export const useAssignedXuiPanelSummariesQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_xui_panel_summaries', resellerId],
    queryFn: () => fetchAssignedXuiPanelSummaries(resellerId as number),
    enabled: resellerId !== undefined,
  })
