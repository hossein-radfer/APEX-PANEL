import { useQuery } from '@tanstack/react-query'
import { fetchReseller } from '@/api/resellers.ts'

export const useResellerQuery = (
  id: number | null | undefined,
  enabled: boolean = true
) =>
  useQuery({
    queryKey: ['reseller', id],
    queryFn: () => fetchReseller(id!),
    enabled: enabled && !!id,
  })
