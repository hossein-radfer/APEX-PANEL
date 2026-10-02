import { useQuery } from '@tanstack/react-query'
import { fetchApplicationsByReseller } from '@/api/application.ts'

export const useApplicationsByResellerQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['applications_list_by_reseller', resellerId],
    queryFn: () => fetchApplicationsByReseller(resellerId as number),
    enabled: resellerId !== undefined,
  })
