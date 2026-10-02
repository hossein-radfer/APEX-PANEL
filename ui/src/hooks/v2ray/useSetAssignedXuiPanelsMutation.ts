import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAssignedXuiPanels } from '@/api/v2ray.ts'

export const useSetAssignedXuiPanelsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAssignedXuiPanels,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['reseller_assigned_xui_panels', variables.resellerId],
      })
    },
  })
}
