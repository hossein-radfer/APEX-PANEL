import { useQuery } from '@tanstack/react-query'
import { fetchLicenseStatus } from '@/api/license.ts'

export const useLicenseStatusQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['license_status'],
    queryFn: fetchLicenseStatus,
    enabled,
    refetchInterval: 5 * 60_000,
  })
