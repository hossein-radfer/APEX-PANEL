import { useQuery } from '@tanstack/react-query'
import { fetchAppVersions } from '@/api/application.ts'

export const useAppVersionsQuery = () =>
  useQuery({
    queryKey: ['app_versions'],
    queryFn: fetchAppVersions,
  })
