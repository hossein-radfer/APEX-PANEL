import { useMutation, useQueryClient } from '@tanstack/react-query'
import { testSavedXuiPanel } from '@/api/xui-panel.ts'

export const useTestSavedXuiPanelMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: testSavedXuiPanel,
    onSuccess: () => {
      // A test call also refreshes the panel's stored status/last_error on
      // the backend, so the list needs to reflect that.
      queryClient.invalidateQueries({ queryKey: ['xui_panels_list'] })
    },
  })
}
