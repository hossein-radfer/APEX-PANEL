import { useQuery } from '@tanstack/react-query'
import { fetchResellers } from '@/api/resellers.ts'

export const useResellersListQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['resellers_list'],
    queryFn: fetchResellers,
    enabled,
  })
