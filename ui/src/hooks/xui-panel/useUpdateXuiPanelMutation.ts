import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateXuiPanel } from '@/api/xui-panel.ts'

export const useUpdateXuiPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateXuiPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['xui_panels_list'] })
    },
  })
}
