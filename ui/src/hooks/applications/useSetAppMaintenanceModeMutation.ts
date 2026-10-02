import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAppMaintenanceMode } from '@/api/application.ts'

export const useSetAppMaintenanceModeMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAppMaintenanceMode,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app_maintenance_mode'] })
    },
  })
}
