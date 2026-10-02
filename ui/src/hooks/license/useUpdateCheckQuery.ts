import { useQuery } from '@tanstack/react-query'
import { checkForUpdate } from '@/api/license.ts'

export const useUpdateCheckQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['update_check'],
    queryFn: () => checkForUpdate('stable'),
    enabled,
    refetchInterval: 60 * 60_000,
    retry: false,
  })
