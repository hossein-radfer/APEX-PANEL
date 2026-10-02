import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteXuiPanel } from '@/api/xui-panel.ts'

export const useDeleteXuiPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteXuiPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['xui_panels_list'] })
    },
  })
}
