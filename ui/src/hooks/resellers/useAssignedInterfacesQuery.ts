import { useQuery } from '@tanstack/react-query'
import { fetchAssignedInterfaces } from '@/api/resellers.ts'

export const useAssignedInterfacesQuery = (resellerId?: number) =>
  useQuery({
    queryKey: ['reseller_assigned_interfaces', resellerId],
    queryFn: () => fetchAssignedInterfaces(resellerId as number),
    enabled: resellerId !== undefined,
  })
