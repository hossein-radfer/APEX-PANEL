import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createXuiPanel } from '@/api/xui-panel.ts'

export const useCreateXuiPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createXuiPanel,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['xui_panels_list'] })
    },
  })
}
