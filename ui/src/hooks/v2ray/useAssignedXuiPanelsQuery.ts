import { useQuery } from '@tanstack/react-query'
import { fetchAssignedXuiPanels } from '@/api/v2ray.ts'

export const useAssignedXuiPanelsQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_xui_panels', resellerId],
    queryFn: () => fetchAssignedXuiPanels(resellerId as number),
    enabled: resellerId !== undefined,
  })
