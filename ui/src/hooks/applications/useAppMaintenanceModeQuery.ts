import { useQuery } from '@tanstack/react-query'
import { fetchAppMaintenanceMode } from '@/api/application.ts'

export const useAppMaintenanceModeQuery = () =>
  useQuery({
    queryKey: ['app_maintenance_mode'],
    queryFn: fetchAppMaintenanceMode,
  })
