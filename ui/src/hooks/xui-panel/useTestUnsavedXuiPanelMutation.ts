import { useMutation } from '@tanstack/react-query'
import { testUnsavedXuiPanel } from '@/api/xui-panel.ts'

export const useTestUnsavedXuiPanelMutation = () =>
  useMutation({
    mutationFn: testUnsavedXuiPanel,
  })
